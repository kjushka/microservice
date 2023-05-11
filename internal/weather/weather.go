package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cenkalti/backoff/v3"
	"github.com/kjushka/microservice-gen/internal/logger"
	"github.com/mercari/go-circuitbreaker"
	"go.opentelemetry.io/otel/trace"
)

type APIWeatherResponse struct {
	CurrentWeather struct {
		Temperature float64 `json:"temperature"`
	} `json:"current_weather"`
}

type APIWeatherClient struct {
	cb *circuitbreaker.CircuitBreaker
}

func NewAPIWeatherClient(ctx context.Context) *APIWeatherClient {
	cb := circuitbreaker.New(
		circuitbreaker.WithClock(clock.New()),
		circuitbreaker.WithFailOnContextCancel(true),
		circuitbreaker.WithFailOnContextDeadline(true),
		circuitbreaker.WithHalfOpenMaxSuccesses(10),
		circuitbreaker.WithOpenTimeoutBackOff(backoff.NewExponentialBackOff()),
		circuitbreaker.WithOpenTimeout(10*time.Second),
		circuitbreaker.WithCounterResetInterval(10*time.Second),
		// we also have NewTripFuncThreshold and NewTripFuncConsecutiveFailures
		circuitbreaker.WithTripFunc(circuitbreaker.NewTripFuncFailureRate(10, 0.4)),
		circuitbreaker.WithOnStateChangeHookFn(func(from, to circuitbreaker.State) {
			logger.Debugf(ctx, "state changed from %s to %s", from, to)
		}),
	)

	return &APIWeatherClient{cb: cb}
}

func (c *APIWeatherClient) CurrentWeather(ctx context.Context) (weather string, err error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	if !c.cb.Ready() {
		return "", circuitbreaker.ErrOpen
	}
	defer func() { err = c.cb.Done(ctx, err) }()

	resp, err := http.Get("https://api.open-meteo.com/v1/forecast?latitude=59.94&longitude=30.31&current_weather=true")
	if err != nil {
		logger.ErrorKV(ctx, "failed making query to weather service", "error", err)
		return "", fmt.Errorf("failed making query to weather service: %v", err)
	}

	data := &APIWeatherResponse{}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(data)
	if err != nil {
		logger.ErrorKV(ctx, "failed unmarshal response", "error", err)
		return "", fmt.Errorf("failed unmarshal response: %v", err)
	}

	return fmt.Sprintf("Temperature in Saint-Petersburg is %.1f C", data.CurrentWeather.Temperature), nil
}
