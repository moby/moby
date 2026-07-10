package tracing

import (
	"context"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc/stats"
)

func ServerStatsHandler(opts ...otelgrpc.Option) stats.Handler {
	handler := otelgrpc.NewServerHandler(opts...)
	return &statsFilter{
		inner:  handler,
		filter: defaultStatsFilter,
	}
}

func ClientStatsHandler(opts ...otelgrpc.Option) stats.Handler {
	handler := otelgrpc.NewClientHandler(opts...)
	return &statsFilter{
		inner:  handler,
		filter: defaultStatsFilter,
	}
}

type contextKey int

const filterContextKey contextKey = iota

type statsFilter struct {
	inner  stats.Handler
	filter func(info *stats.RPCTagInfo) bool
}

func (s *statsFilter) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	if s.filter(info) {
		return context.WithValue(ctx, filterContextKey, struct{}{})
	}
	return s.inner.TagRPC(ctx, info)
}

func (s *statsFilter) HandleRPC(ctx context.Context, rpcStats stats.RPCStats) {
	if ctx.Value(filterContextKey) != nil {
		return
	}
	s.inner.HandleRPC(ctx, rpcStats)
}

func (s *statsFilter) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	return s.inner.TagConn(ctx, info)
}

func (s *statsFilter) HandleConn(ctx context.Context, connStats stats.ConnStats) {
	s.inner.HandleConn(ctx, connStats)
}

func defaultStatsFilter(info *stats.RPCTagInfo) bool {
	return strings.HasSuffix(info.FullMethodName, "opentelemetry.proto.collector.trace.v1.TraceService/Export") ||
		strings.HasSuffix(info.FullMethodName, "Health/Check")
}
