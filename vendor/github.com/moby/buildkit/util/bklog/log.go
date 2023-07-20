package bklog

import (
	"context"
	"runtime/debug"

	"github.com/containerd/containerd/log"
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
	} else if logger := log.GetLogger(ctx); logger != nil {
		l = logger
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

// LazyStackTrace lets you include a stack trace as a field's value in a log but only
// call it when the log level is actually enabled.
type LazyStackTrace struct{}

func (LazyStackTrace) String() string {
	return string(debug.Stack())
}

func (LazyStackTrace) MarshalText() ([]byte, error) {
	return debug.Stack(), nil
}
