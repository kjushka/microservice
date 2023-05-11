package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"syscall"
	"time"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/kjushka/microservice-gen/internal/config"
	"github.com/kjushka/microservice-gen/internal/errgroup"
	"github.com/kjushka/microservice-gen/internal/logger"
	"github.com/kjushka/microservice-gen/internal/migrator"
	"github.com/kjushka/microservice-gen/internal/ratelimiter"
	"github.com/kjushka/microservice-gen/internal/storage/cache"
	"github.com/kjushka/microservice-gen/internal/storage/database"
	"github.com/kjushka/microservice-gen/internal/storage/storage-with-cache"
	"github.com/kjushka/microservice-gen/internal/tracing"
	microservicepb2 "github.com/kjushka/microservice-gen/pkg/microservice"
	"github.com/kjushka/microservice-gen/service"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const serviceName = "some service"

// interceptorLogger adapts go-kit logger to interceptor logger.
// This code is simple enough to be copied and not imported.
func interceptorLogger() logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		largs := append([]any{"msg", msg}, fields...)
		switch lvl {
		case logging.LevelDebug:
			logger.Debug(ctx, largs...)
		case logging.LevelInfo:
			logger.Info(ctx, largs...)
		case logging.LevelWarn:
			logger.Warn(ctx, largs...)
		case logging.LevelError:
			logger.Error(ctx, largs...)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}

func main() {
	ctx := context.Background()
	logger.SetLevel(zapcore.DebugLevel)

	tracer, err := tracing.InitTracer("http://jaeger:14268/api/traces", serviceName)
	if err != nil {
		logger.FatalKV(ctx, "init tracer", "error", err)
	}

	logTraceID := func(ctx context.Context) logging.Fields {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return logging.Fields{"traceID", span.TraceID().String()}
		}
		return nil
	}

	cfg, err := config.InitConfig(ctx)
	if err != nil {
		logger.PanicKV(ctx, "failed config initiating", "error", err)
	}

	db, err := database.InitDB(ctx, cfg, tracer)
	if err != nil {
		logger.PanicKV(ctx, "failed create database conn", "error", err)
	}

	err = migrator.Migrate(db.GetDB(), cfg)
	if err != nil {
		logger.PanicKV(ctx, "failed migrate process", "error", err)
	}

	redisCache, err := cache.InitCache(cfg, tracer)
	if err != nil {
		logger.PanicKV(ctx, "failed cache initiating", "error", err)
	}

	storage := storage_with_cache.NewStorage(redisCache, db, tracer)

	srvMetrics := grpcprom.NewServerMetrics(
		grpcprom.WithServerHandlingTimeHistogram(
			grpcprom.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120}),
		),
	)
	clMetrics := grpcprom.NewClientMetrics(
		grpcprom.WithClientHandlingTimeHistogram(
			grpcprom.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120}),
		),
	)

	reg := prometheus.NewRegistry()
	reg.MustRegister(srvMetrics)
	exemplarFromContext := func(ctx context.Context) prometheus.Labels {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return prometheus.Labels{"traceID": span.TraceID().String()}
		}
		return nil
	}

	// Setup custom auth.
	authFn := func(ctx context.Context) (context.Context, error) {
		token, err := auth.AuthFromMD(ctx, "bearer")
		if err != nil {
			return nil, err
		}
		// TODO: This is example only, perform proper Oauth/OIDC verification!
		if token != "yolo" {
			return nil, status.Errorf(codes.Unauthenticated, "invalid auth token")
		}
		// NOTE: You can also pass the token in the context for further interceptors or gRPC service code.
		return ctx, nil
	}

	// Setup auth matcher.
	allButHealthZ := func(ctx context.Context, callMeta interceptors.CallMeta) bool {
		return healthpb.Health_ServiceDesc.ServiceName != callMeta.Service
	}

	// Setup metric for panic recoveries.
	panicsTotal := promauto.With(reg).NewCounter(prometheus.CounterOpts{
		Name: "grpc_req_panics_recovered_total",
		Help: "Total number of gRPC requests recovered from internal panic.",
	})

	grpcPanicRecoveryHandler := func(p any) (err error) {
		panicsTotal.Inc()
		logger.Error(ctx, "msg", "recovered from panic", "panic", p, "stack", debug.Stack())
		return status.Errorf(codes.Internal, "%s", p)
	}

	// Create a gRPC server object
	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			otelgrpc.UnaryServerInterceptor(),
			srvMetrics.UnaryServerInterceptor(grpcprom.WithExemplarFromContext(exemplarFromContext)),
			logging.UnaryServerInterceptor(interceptorLogger(), logging.WithFieldsFromContext(logTraceID)),
			selector.UnaryServerInterceptor(auth.UnaryServerInterceptor(authFn), selector.MatchFunc(allButHealthZ)),
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler)),
			ratelimiter.UnaryServerInterceptor(redisCache.RedisClient(), cfg),
		),
	)
	// Attach the Greeter service to the server
	microservicepb2.RegisterHTTPMicroserviceServer(s, service.NewHandler(ctx, storage, tracer))

	group, ctx := errgroup.WithContext(ctx)

	// Serve gRPC server
	group.Go(func() error {
		// Create a listener on TCP port
		lis, err := net.Listen("tcp", ":8080")
		if err != nil {
			logger.PanicKV(ctx, "failed to listen :8080", "error", err)
		}

		logger.Info(ctx, "starting gRPC server", "addr", lis.Addr().String())
		return s.Serve(lis)
	})

	httpSrv := &http.Server{Addr: ":8081"}
	group.Go(func() error {
		m := http.NewServeMux()
		// Create HTTP handler for Prometheus metrics.
		m.Handle("/metrics", promhttp.HandlerFor(
			reg,
			promhttp.HandlerOpts{
				// Opt into OpenMetrics e.g. to support exemplars.
				EnableOpenMetrics: true,
			},
		))
		httpSrv.Handler = m
		logger.Info(ctx, "starting HTTP server", "addr", httpSrv.Addr)
		return httpSrv.ListenAndServe()
	})

	sigExec, sigInterr := run.SignalHandler(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	group.Go(sigExec)

	// Create a client connection to the gRPC server we just started
	// This is where the gRPC-Gateway proxies the requests
	conn, err := grpc.DialContext(
		context.Background(),
		"0.0.0.0:8080",
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			otelgrpc.UnaryClientInterceptor(),
			clMetrics.UnaryClientInterceptor(grpcprom.WithExemplarFromContext(exemplarFromContext)),
			logging.UnaryClientInterceptor(interceptorLogger(), logging.WithFieldsFromContext(logTraceID)),
			retry.UnaryClientInterceptor(retry.WithMax(5), retry.WithPerRetryTimeout(time.Millisecond*100)),
		),
	)
	if err != nil {
		logger.PanicKV(ctx, "failed to dial server 8090", "error", err)
	}

	gwmux := runtime.NewServeMux()
	err = microservicepb2.RegisterHTTPMicroserviceHandler(ctx, gwmux, conn)
	if err != nil {
		logger.PanicKV(ctx, "failed to register gateway", "error", err)
	}

	gwServer := &http.Server{
		Addr:    ":8090",
		Handler: gwmux,
	}

	group.Go(func() error {
		logger.Info(ctx, "Serving gRPC-Gateway on http://0.0.0.0:8090")
		return gwServer.ListenAndServe()
	})

	if err = group.Wait(); err != nil {
		s.GracefulStop()
		s.Stop()
		if err := httpSrv.Close(); err != nil {
			logger.Error(ctx, "failed to stop web server", "err", err)
		}
		sigInterr(err)
		logger.PanicKV(ctx, "have some error", "error", err)
	}
}
