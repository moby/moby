package tracing

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/opentracing-contrib/go-stdlib/nethttp"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
)

// StartSpan starts a new span as a child of the span in context.
// If there is no span in context then this is a no-op.
// The difference from opentracing.StartSpanFromContext is that this method
// does not depend on global tracer.
func StartSpan(ctx context.Context, operationName string, opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context) {
	parent := opentracing.SpanFromContext(ctx)
	tracer := opentracing.Tracer(&opentracing.NoopTracer{})
	if parent != nil {
		tracer = parent.Tracer()
		opts = append(opts, opentracing.ChildOf(parent.Context()))
	}
	span := tracer.StartSpan(operationName, opts...)
	if parent != nil {
		return span, opentracing.ContextWithSpan(ctx, span)
	}
	return span, ctx
}

// FinishWithError finalizes the span and sets the error if one is passed
func FinishWithError(span opentracing.Span, err error) {
	if err != nil {
		fields := []log.Field{
			log.String("event", "error"),
			log.String("message", err.Error()),
		}
		if _, ok := err.(interface {
			Cause() error
		}); ok {
			fields = append(fields, log.String("stack", fmt.Sprintf("%+v", err)))
		}
		span.LogFields(fields...)
		ext.Error.Set(span, true)
	}
	span.Finish()
}

// ContextWithSpanFromContext sets the tracing span of a context from other
// context if one is not already set. Alternative would be
// context.WithoutCancel() that would copy the context but reset ctx.Done
func ContextWithSpanFromContext(ctx, ctx2 context.Context) context.Context {
	// if already is a span then noop
	if span := opentracing.SpanFromContext(ctx); span != nil {
		return ctx
	}
	if span := opentracing.SpanFromContext(ctx2); span != nil {
		return opentracing.ContextWithSpan(ctx, span)
	}
	return ctx
}

var DefaultTransport http.RoundTripper = &Transport{
	RoundTripper: &nethttp.Transport{RoundTripper: http.DefaultTransport},
}

var DefaultClient = &http.Client{
	Transport: DefaultTransport,
}

type Transport struct {
	http.RoundTripper
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	span := opentracing.SpanFromContext(req.Context())
	if span == nil { // no tracer connected with either request or transport
		return t.RoundTripper.RoundTrip(req)
	}

	req, tracer := nethttp.TraceRequest(span.Tracer(), req)

	resp, err := t.RoundTripper.RoundTrip(req)
	if err != nil {
		tracer.Finish()
		return resp, err
	}

	if req.Method == "HEAD" {
		tracer.Finish()
	} else {
		resp.Body = closeTracker{resp.Body, tracer.Finish}
	}

	return resp, err
}

type closeTracker struct {
	io.ReadCloser
	finish func()
}

func (c closeTracker) Close() error {
	err := c.ReadCloser.Close()
	c.finish()
	return err
}
