package logger

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"go.uber.org/zap"
)

type contextKey int

const (
	loggerContextKey contextKey = iota
)

// ToContext создает контекст с переданным логгером внутри.
func ToContext(ctx context.Context, l *zap.SugaredLogger) context.Context {
	return context.WithValue(ctx, loggerContextKey, l)
}

// FromContext достает логгер из контекста. Если в контексте логгер не
// обнаруживается - возвращает глобальный логгер. В обоих случаях логгер уже
// содержит аннотации в виде trace_id и span_id
func FromContext(ctx context.Context) *zap.SugaredLogger {
	l := global

	if logger, ok := ctx.Value(loggerContextKey).(*zap.SugaredLogger); ok {
		l = logger
	}

	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		return l
	}

	return loggerWithSpanContext(l, span.Context())
}

// FromContextLT ...
// Deprecated: long-term аннотации больше не актуальны, используйте FromContext
func FromContextLT(ctx context.Context) *zap.SugaredLogger {
	return FromContext(ctx)
}

func loggerWithSpanContext(l *zap.SugaredLogger, sc opentracing.SpanContext) *zap.SugaredLogger {
	if sc, ok := sc.(jaeger.SpanContext); ok {
		return l.Desugar().With(
			zap.Stringer("trace_id", sc.TraceID()),
			zap.Stringer("span_id", sc.SpanID()),
		).Sugar()
	}

	return l
}

// WithName создает именованный логгер из уже имеющегося в контексте.
// Дочерние логгеры будут наследовать имена (см пример).
func WithName(ctx context.Context, name string) context.Context {
	log := FromContext(ctx).Named(name)
	return ToContext(ctx, log)
}

// WithKV создает логгер из уже имеющегося в контексте и устанавливает метаданные.
// Принимает ключ и значение, которые будут наследоваться дочерними логгерами.
func WithKV(ctx context.Context, key string, value interface{}) context.Context {
	log := FromContext(ctx).With(key, value)
	return ToContext(ctx, log)
}

// WithFields создает логгер из уже имеющегося в контексте и устанавливает метаданные,
// используя типизированные поля.
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	log := FromContext(ctx).
		Desugar().
		With(fields...).
		Sugar()
	return ToContext(ctx, log)
}
