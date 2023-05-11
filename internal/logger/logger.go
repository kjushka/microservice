package logger

import (
	"context"
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// global глобальный экземпляр логгера.
	global       *zap.SugaredLogger
	defaultLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
)

func init() {
	SetLogger(New(defaultLevel))
}

// New создает экземпляр *zap.SugaredLogger cо стандартным json выводом.
// Если уровень логгирования не передан - будет использоваться уровень
// по умолчанию (zap.ErrorLevel)
func New(level zapcore.LevelEnabler, options ...zap.Option) *zap.SugaredLogger {
	return NewWithSink(level, os.Stdout, options...)
}

// NewWithSink создает экземпляр *zap.SugaredLogger cо стандартным json выводом.
// Если уровень логгирования не передан - будет использоваться уровень
// по умолчанию (zap.ErrorLevel). Sink используется для вывода логов.
func NewWithSink(level zapcore.LevelEnabler, sink io.Writer, options ...zap.Option) *zap.SugaredLogger {
	if level == nil {
		level = defaultLevel
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}),
		zapcore.AddSync(sink),
		level,
	)

	return zap.New(core, options...).Sugar()
}

// Level возвращает текущий уровень логгирования глобального логгера.
func Level() zapcore.Level {
	return defaultLevel.Level()
}

// SetLevel устанавливает уровень логгирования глобального логгера.
func SetLevel(l zapcore.Level) {
	defaultLevel.SetLevel(l)
}

// Logger возвращает глобальный логгер.
func Logger() *zap.SugaredLogger {
	return global
}

// SetLogger устанавливает глобальный логгер. Функция непотокобезопасна.
func SetLogger(l *zap.SugaredLogger) {
	global = l
}

func Debug(ctx context.Context, args ...interface{}) {
	FromContext(ctx).Debug(args...)
}

func Debugf(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Debugf(format, args...)
}

func DebugKV(ctx context.Context, message string, kvs ...interface{}) {
	FromContext(ctx).Debugw(message, kvs...)
}

func Info(ctx context.Context, args ...interface{}) {
	FromContext(ctx).Info(args...)
}

func Infof(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Infof(format, args...)
}

func InfoKV(ctx context.Context, message string, kvs ...interface{}) {
	FromContext(ctx).Infow(message, kvs...)
}

func Warn(ctx context.Context, args ...interface{}) {
	FromContext(ctx).Warn(args...)
}

func Warnf(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Warnf(format, args...)
}

func WarnKV(ctx context.Context, message string, kvs ...interface{}) {
	FromContext(ctx).Warnw(message, kvs...)
}

func Error(ctx context.Context, args ...interface{}) {
	FromContext(ctx).Error(args...)
}

func Errorf(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Errorf(format, args...)
}

func ErrorKV(ctx context.Context, message string, kvs ...interface{}) {
	FromContext(ctx).Errorw(message, kvs...)
}

func Fatal(ctx context.Context, args ...interface{}) {
	FromContext(ctx).Fatal(args...)
}

func Fatalf(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Fatalf(format, args...)
}

func FatalKV(ctx context.Context, message string, kvs ...interface{}) {
	FromContext(ctx).Fatalw(message, kvs...)
}

func Panic(ctx context.Context, args ...interface{}) {
	FromContext(ctx).Panic(args...)
}

func Panicf(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Panicf(format, args...)
}

func PanicKV(ctx context.Context, message string, kvs ...interface{}) {
	FromContext(ctx).Panicw(message, kvs...)
}
