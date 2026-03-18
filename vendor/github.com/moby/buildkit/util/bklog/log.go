package bklog

import (
	"context"
	"runtime/debug"

	"github.com/containerd/log"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	// overwrites containerd/log
	log.G = GetLogger
	log.L = L
}

var (
	G = GetLogger
	L = logrus.NewEntry(logrus.StandardLogger())
)

type (
	loggerKey struct{}
)

// WithLogger returns a new context with the provided logger. Use in
// combination with logger.WithField(s) for great effect.
func WithLogger(ctx context.Context, logger *logrus.Entry) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// GetLogger retrieves the current logger from the context. If no logger is
// available, the default logger is returned.
func GetLogger(ctx context.Context) (l *logrus.Entry) {
	logger := ctx.Value(loggerKey{})

	if logger != nil {
		l = logger.(*logrus.Entry)
	} else if logger := log.GetLogger(ctx); logger != nil {
		l = logger
	} else {
		l = L
	}

	if spanContext := trace.SpanFromContext(ctx).SpanContext(); spanContext.IsValid() {
		return l.WithFields(logrus.Fields{
			"traceID": spanContext.TraceID(),
			"spanID":  spanContext.SpanID(),
		})
	}

	return l
}

// TraceLevelOnlyStack returns a stack trace for the current goroutine only if
// trace level logs are enabled; otherwise it returns an empty string. This ensure
// we only pay the cost of generating a stack trace when the log entry will actually
// be emitted.
func TraceLevelOnlyStack() string {
	if logrus.GetLevel() == logrus.TraceLevel {
		return string(debug.Stack())
	}
	return ""
}
