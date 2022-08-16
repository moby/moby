package tracing

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptrace"

	"github.com/moby/buildkit/util/bklog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

// StartSpan starts a new span as a child of the span in context.
// If there is no span in context then this is a no-op.
func StartSpan(ctx context.Context, operationName string, opts ...trace.SpanStartOption) (trace.Span, context.Context) {
	parent := trace.SpanFromContext(ctx)
	tracer := trace.NewNoopTracerProvider().Tracer("")
	if parent != nil && parent.SpanContext().IsValid() {
		tracer = parent.TracerProvider().Tracer("")
	}
	ctx, span := tracer.Start(ctx, operationName, opts...)
	ctx = bklog.WithLogger(ctx, bklog.GetLogger(ctx).WithField("span", operationName))
	return span, ctx
}

// FinishWithError finalizes the span and sets the error if one is passed
func FinishWithError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		if _, ok := err.(interface {
			Cause() error
		}); ok {
			span.SetAttributes(attribute.String(string(semconv.ExceptionStacktraceKey), fmt.Sprintf("%+v", err)))
		}
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// ContextWithSpanFromContext sets the tracing span of a context from other
// context if one is not already set. Alternative would be
// context.WithoutCancel() that would copy the context but reset ctx.Done
func ContextWithSpanFromContext(ctx, ctx2 context.Context) context.Context {
	// if already is a span then noop
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		return ctx
	}
	if span := trace.SpanFromContext(ctx2); span != nil && span.SpanContext().IsValid() {
		return trace.ContextWithSpan(ctx, span)
	}
	return ctx
}

var DefaultTransport = NewTransport(http.DefaultTransport)

var DefaultClient = &http.Client{
	Transport: DefaultTransport,
}

func NewTransport(rt http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(rt,
		otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})),
		otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
			return otelhttptrace.NewClientTrace(ctx, otelhttptrace.WithoutSubSpans())
		}),
	)
}
