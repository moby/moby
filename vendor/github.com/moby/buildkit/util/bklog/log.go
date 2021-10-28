package bklog

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

var (
	G = GetLogger
	L = logrus.NewEntry(logrus.StandardLogger())
)

var (
	logWithTraceID = false
)

func EnableLogWithTraceID(b bool) {
	logWithTraceID = b
}

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
	} else {
		l = L
	}

	if logWithTraceID {
		if spanContext := trace.SpanFromContext(ctx).SpanContext(); spanContext.IsValid() {
			return l.WithFields(logrus.Fields{
				"traceID": spanContext.TraceID(),
				"spanID":  spanContext.SpanID(),
			})
		}
	}

	return l
}
