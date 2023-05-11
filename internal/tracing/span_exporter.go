package tracing

import (
	"go.opentelemetry.io/otel/exporters/jaeger"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

// newJaegerExporter creates new jaeger exporter
//
//	url example - http://localhost:14268/api/traces
func newJaegerExporter(url string) (tracesdk.SpanExporter, error) {
	return jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
}
