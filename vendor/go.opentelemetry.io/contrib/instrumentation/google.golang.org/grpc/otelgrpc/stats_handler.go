// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelgrpc // import "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

import (
	"context"
	"sync/atomic"
	"time"

	grpc_codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/internal"
)

type gRPCContextKey struct{}

type gRPCContext struct {
	inMessages  int64
	outMessages int64
	metricAttrs []attribute.KeyValue
	record      bool
}

type serverHandler struct {
	*config

	tracer trace.Tracer

	duration metric.Float64Histogram
	inSize   metric.Int64Histogram
	outSize  metric.Int64Histogram
	inMsg    metric.Int64Histogram
	outMsg   metric.Int64Histogram
}

// NewServerHandler creates a stats.Handler for a gRPC server.
func NewServerHandler(opts ...Option) stats.Handler {
	c := newConfig(opts)
	h := &serverHandler{config: c}

	h.tracer = c.TracerProvider.Tracer(
		ScopeName,
		trace.WithInstrumentationVersion(Version()),
	)

	meter := c.MeterProvider.Meter(
		ScopeName,
		metric.WithInstrumentationVersion(Version()),
		metric.WithSchemaURL(semconv.SchemaURL),
	)

	var err error
	h.duration, err = meter.Float64Histogram(
		semconv.RPCServerDurationName,
		metric.WithDescription(semconv.RPCServerDurationDescription),
		metric.WithUnit(semconv.RPCServerDurationUnit),
	)
	if err != nil {
		otel.Handle(err)
		if h.duration == nil {
			h.duration = noop.Float64Histogram{}
		}
	}

	h.inSize, err = meter.Int64Histogram(
		semconv.RPCServerRequestSizeName,
		metric.WithDescription(semconv.RPCServerRequestSizeDescription),
		metric.WithUnit(semconv.RPCServerRequestSizeUnit),
	)
	if err != nil {
		otel.Handle(err)
		if h.inSize == nil {
			h.inSize = noop.Int64Histogram{}
		}
	}

	h.outSize, err = meter.Int64Histogram(
		semconv.RPCServerResponseSizeName,
		metric.WithDescription(semconv.RPCServerResponseSizeDescription),
		metric.WithUnit(semconv.RPCServerResponseSizeUnit),
	)
	if err != nil {
		otel.Handle(err)
		if h.outSize == nil {
			h.outSize = noop.Int64Histogram{}
		}
	}

	h.inMsg, err = meter.Int64Histogram(
		semconv.RPCServerRequestsPerRPCName,
		metric.WithDescription(semconv.RPCServerRequestsPerRPCDescription),
		metric.WithUnit(semconv.RPCServerRequestsPerRPCUnit),
	)
	if err != nil {
		otel.Handle(err)
		if h.inMsg == nil {
			h.inMsg = noop.Int64Histogram{}
		}
	}

	h.outMsg, err = meter.Int64Histogram(
		semconv.RPCServerResponsesPerRPCName,
		metric.WithDescription(semconv.RPCServerResponsesPerRPCDescription),
		metric.WithUnit(semconv.RPCServerResponsesPerRPCUnit),
	)
	if err != nil {
		otel.Handle(err)
		if h.outMsg == nil {
			h.outMsg = noop.Int64Histogram{}
		}
	}

	return h
}

// TagConn can attach some information to the given context.
func (h *serverHandler) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	return ctx
}

// HandleConn processes the Conn stats.
func (h *serverHandler) HandleConn(ctx context.Context, info stats.ConnStats) {
}

// TagRPC can attach some information to the given context.
func (h *serverHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	ctx = extract(ctx, h.Propagators)

	name, attrs := internal.ParseFullMethod(info.FullMethodName)
	attrs = append(attrs, semconv.RPCSystemGRPC)

	record := true
	if h.Filter != nil {
		record = h.Filter(info)
	}

	if record {
		ctx, _ = h.tracer.Start(
			trace.ContextWithRemoteSpanContext(ctx, trace.SpanContextFromContext(ctx)),
			name,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(append(attrs, h.SpanAttributes...)...),
		)
	}

	gctx := gRPCContext{
		metricAttrs: append(attrs, h.MetricAttributes...),
		record:      record,
	}

	return context.WithValue(ctx, gRPCContextKey{}, &gctx)
}

// HandleRPC processes the RPC stats.
func (h *serverHandler) HandleRPC(ctx context.Context, rs stats.RPCStats) {
	h.handleRPC(ctx, rs, h.duration, h.inSize, h.outSize, h.inMsg, h.outMsg, serverStatus)
}

type clientHandler struct {
	*config

	tracer trace.Tracer

	duration metric.Float64Histogram
	inSize   metric.Int64Histogram
	outSize  metric.Int64Histogram
	inMsg    metric.Int64Histogram
	outMsg   metric.Int64Histogram
}

// NewClientHandler creates a stats.Handler for a gRPC client.
func NewClientHandler(opts ...Option) stats.Handler {
	c := newConfig(opts)
	h := &clientHandler{config: c}

	h.tracer = c.TracerProvider.Tracer(
		ScopeName,
		trace.WithInstrumentationVersion(Version()),
	)

	meter := c.MeterProvider.Meter(
		ScopeName,
		metric.WithInstrumentationVersion(Version()),
		metric.WithSchemaURL(semconv.SchemaURL),
	)

	var err error
	h.duration, err = meter.Float64Histogram(
		semconv.RPCClientDurationName,
		metric.WithDescription(semconv.RPCClientDurationDescription),
		metric.WithUnit(semconv.RPCClientDurationUnit),
	)
	if err != nil {
		otel.Handle(err)
		if h.duration == nil {
			h.duration = noop.Float64Histogram{}
		}
	}

	h.outSize, err = meter.Int64Histogram(
		semconv.RPCClientRequestSizeName,
		metric.WithDescription(semconv.RPCClientRequestSizeDescription),
		metric.WithUnit(semconv.RPCClientRequestSizeUnit),
	)
	if err != nil {
		otel.Handle(err)
		if h.outSize == nil {
			h.outSize = noop.Int64Histogram{}
		}
	}

	h.inSize, err = meter.Int64Histogram(
		semconv.RPCClientResponseSizeName,
		metric.WithDescription(semconv.RPCClientResponseSizeDescription),
		metric.WithUnit(semconv.RPCClientResponseSizeUnit),
	)
	if err != nil {
		otel.Handle(err)
		if h.inSize == nil {
			h.inSize = noop.Int64Histogram{}
		}
	}

	h.outMsg, err = meter.Int64Histogram(
		semconv.RPCClientRequestsPerRPCName,
		metric.WithDescription(semconv.RPCClientRequestsPerRPCDescription),
		metric.WithUnit(semconv.RPCClientRequestsPerRPCUnit),
	)
	if err != nil {
		otel.Handle(err)
		if h.outMsg == nil {
			h.outMsg = noop.Int64Histogram{}
		}
	}

	h.inMsg, err = meter.Int64Histogram(
		semconv.RPCClientResponsesPerRPCName,
		metric.WithDescription(semconv.RPCClientResponsesPerRPCDescription),
		metric.WithUnit(semconv.RPCClientResponsesPerRPCUnit),
	)
	if err != nil {
		otel.Handle(err)
		if h.inMsg == nil {
			h.inMsg = noop.Int64Histogram{}
		}
	}

	return h
}

// TagRPC can attach some information to the given context.
func (h *clientHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	name, attrs := internal.ParseFullMethod(info.FullMethodName)
	attrs = append(attrs, semconv.RPCSystemGRPC)

	record := true
	if h.Filter != nil {
		record = h.Filter(info)
	}

	if record {
		ctx, _ = h.tracer.Start(
			ctx,
			name,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(append(attrs, h.SpanAttributes...)...),
		)
	}

	gctx := gRPCContext{
		metricAttrs: append(attrs, h.MetricAttributes...),
		record:      record,
	}

	return inject(context.WithValue(ctx, gRPCContextKey{}, &gctx), h.Propagators)
}

// HandleRPC processes the RPC stats.
func (h *clientHandler) HandleRPC(ctx context.Context, rs stats.RPCStats) {
	h.handleRPC(
		ctx, rs, h.duration, h.inSize, h.outSize, h.inMsg, h.outMsg,
		func(s *status.Status) (codes.Code, string) {
			return codes.Error, s.Message()
		},
	)
}

// TagConn can attach some information to the given context.
func (h *clientHandler) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	return ctx
}

// HandleConn processes the Conn stats.
func (h *clientHandler) HandleConn(context.Context, stats.ConnStats) {
	// no-op
}

func (c *config) handleRPC(
	ctx context.Context,
	rs stats.RPCStats,
	duration metric.Float64Histogram,
	inSize, outSize, inMsg, outMsg metric.Int64Histogram,
	recordStatus func(*status.Status) (codes.Code, string),
) {
	gctx, _ := ctx.Value(gRPCContextKey{}).(*gRPCContext)
	if gctx != nil && !gctx.record {
		return
	}

	span := trace.SpanFromContext(ctx)
	var messageId int64

	switch rs := rs.(type) {
	case *stats.Begin:
	case *stats.InPayload:
		if gctx != nil {
			messageId = atomic.AddInt64(&gctx.inMessages, 1)
			inSize.Record(ctx, int64(rs.Length), metric.WithAttributes(gctx.metricAttrs...))
		}

		if c.ReceivedEvent && span.IsRecording() {
			span.AddEvent("message",
				trace.WithAttributes(
					semconv.RPCMessageTypeReceived,
					semconv.RPCMessageIDKey.Int64(messageId),
					semconv.RPCMessageCompressedSizeKey.Int(rs.CompressedLength),
					semconv.RPCMessageUncompressedSizeKey.Int(rs.Length),
				),
			)
		}
	case *stats.OutPayload:
		if gctx != nil {
			messageId = atomic.AddInt64(&gctx.outMessages, 1)
			outSize.Record(ctx, int64(rs.Length), metric.WithAttributes(gctx.metricAttrs...))
		}

		if c.SentEvent && span.IsRecording() {
			span.AddEvent("message",
				trace.WithAttributes(
					semconv.RPCMessageTypeSent,
					semconv.RPCMessageIDKey.Int64(messageId),
					semconv.RPCMessageCompressedSizeKey.Int(rs.CompressedLength),
					semconv.RPCMessageUncompressedSizeKey.Int(rs.Length),
				),
			)
		}
	case *stats.OutTrailer:
	case *stats.OutHeader:
		if span.IsRecording() {
			if p, ok := peer.FromContext(ctx); ok {
				span.SetAttributes(serverAddrAttrs(p.Addr.String())...)
			}
		}
	case *stats.End:
		var rpcStatusAttr attribute.KeyValue

		var s *status.Status
		if rs.Error != nil {
			s, _ = status.FromError(rs.Error)
			rpcStatusAttr = semconv.RPCGRPCStatusCodeKey.Int(int(s.Code()))
		} else {
			rpcStatusAttr = semconv.RPCGRPCStatusCodeKey.Int(int(grpc_codes.OK))
		}
		if span.IsRecording() {
			if s != nil {
				c, m := recordStatus(s)
				span.SetStatus(c, m)
			}
			span.SetAttributes(rpcStatusAttr)
			span.End()
		}

		var metricAttrs []attribute.KeyValue
		if gctx != nil {
			metricAttrs = make([]attribute.KeyValue, 0, len(gctx.metricAttrs)+1)
			metricAttrs = append(metricAttrs, gctx.metricAttrs...)
		}
		metricAttrs = append(metricAttrs, rpcStatusAttr)
		// Allocate vararg slice once.
		recordOpts := []metric.RecordOption{metric.WithAttributeSet(attribute.NewSet(metricAttrs...))}

		// Use floating point division here for higher precision (instead of Millisecond method).
		// Measure right before calling Record() to capture as much elapsed time as possible.
		elapsedTime := float64(rs.EndTime.Sub(rs.BeginTime)) / float64(time.Millisecond)

		duration.Record(ctx, elapsedTime, recordOpts...)
		if gctx != nil {
			inMsg.Record(ctx, atomic.LoadInt64(&gctx.inMessages), recordOpts...)
			outMsg.Record(ctx, atomic.LoadInt64(&gctx.outMessages), recordOpts...)
		}
	default:
		return
	}
}
