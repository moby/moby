package log

import (
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// Hook serves to intercept and format `logrus.Entry`s before they are passed
// to the ETW hook.
//
// The containerd shim discards the (formatted) logrus output, and outputs only via ETW.
// The Linux GCS outputs logrus entries over stdout, which is consumed by the shim and
// then re-output via the ETW hook.
type Hook struct{}

var _ logrus.Hook = &Hook{}

func NewHook() *Hook {
	return &Hook{}
}

func (h *Hook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *Hook) Fire(e *logrus.Entry) (err error) {
	h.addSpanContext(e)

	return nil
}

func (h *Hook) addSpanContext(e *logrus.Entry) {
	ctx := e.Context
	if ctx == nil {
		return
	}
	span := trace.FromContext(ctx)
	if span == nil {
		return
	}
	sctx := span.SpanContext()
	e.Data[logfields.TraceID] = sctx.TraceID.String()
	e.Data[logfields.SpanID] = sctx.SpanID.String()
}
