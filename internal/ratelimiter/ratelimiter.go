package ratelimiter

import (
	"context"
	"time"

	"github.com/kjushka/microservice-gen/internal/config"
	"github.com/kjushka/microservice-gen/internal/logger"
	"github.com/mennanov/limiters"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func UnaryServerInterceptor(redisClient *redis.Client, cfg *config.Config) grpc.UnaryServerInterceptor {
	rate := time.Second * 3
	limiter := limiters.NewFixedWindow(
		cfg.RateLimiterCapacity,
		rate,
		limiters.NewFixedWindowRedis(redisClient, "rate-limiting"),
		limiters.NewSystemClock(),
	)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		w, err := limiter.Limit(ctx)
		if err == limiters.ErrLimitExhausted {
			return nil, status.Errorf(codes.ResourceExhausted, "try again later in %s", w)
		} else if err != nil {
			// The limiter failed. This error should be logged and examined.
			logger.ErrorKV(ctx, "limiter failed", "error", err)
			return nil, status.Error(codes.Internal, "internal error")
		}
		return handler(ctx, req)
	}
}
