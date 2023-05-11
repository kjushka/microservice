package service

import (
	"context"
	"fmt"
	"github.com/kjushka/microservice-gen/internal/storage"
	"github.com/kjushka/microservice-gen/internal/weather"
	microservicepb2 "github.com/kjushka/microservice-gen/pkg/microservice"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Handler struct {
	microservicepb2.HTTPMicroserviceServer

	storage       storage.Storage
	tracer        trace.Tracer
	weatherClient *weather.APIWeatherClient
}

func NewHandler(ctx context.Context, storage storage.Storage, tracer trace.Tracer) *Handler {
	return &Handler{
		storage:       storage,
		tracer:        tracer,
		weatherClient: weather.NewAPIWeatherClient(ctx),
	}
}

func (h *Handler) Welcome(ctx context.Context, _ *emptypb.Empty) (*microservicepb2.WelcomeResponse, error) {
	ctx, span := h.tracer.Start(ctx, "welcome")
	defer span.End()

	temperatureInfo, err := h.weatherClient.CurrentWeather(ctx)
	if err != nil {
		return nil, err
	}

	return &microservicepb2.WelcomeResponse{Message: fmt.Sprintf("Welcome! %s", temperatureInfo)}, nil
}
