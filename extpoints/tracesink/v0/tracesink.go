// Package tracesinkv0 defines a point for receiving dockerd trace batches.
package tracesinkv0

import (
	"context"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/extensions/serverpoint"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

// Sink receives batches of sampled daemon spans using the OTLP trace protocol.
// Implementations must honor context cancellation and must not generate spans
// on the daemon's tracer provider while processing exports.
type Sink interface {
	ExportTraces(context.Context, *tracepb.ExportTraceServiceRequest) (*tracepb.ExportTraceServiceResponse, error)
}

// Point allows multiple trace sinks.
var Point = extensions.DefinePoint[Sink]("org.mobyproject.extension.telemetry.tracesink.v0")

// ServerPoint serves the standard OTLP trace service on the extension server.
var ServerPoint = serverpoint.Registration{
	Point: Point.ID(),
	Register: func(r grpc.ServiceRegistrar, impl any) {
		tracepb.RegisterTraceServiceServer(r, traceServer{Sink: impl.(Sink)})
	},
}

// ClientPoint uses the host-owned connection to the extension.
var ClientPoint = clientpoint.Registration{
	Point: Point.ID(),
	Provider: func(conn grpc.ClientConnInterface) extensions.Provider {
		return Point.Provide(traceClient{client: tracepb.NewTraceServiceClient(conn)})
	},
}

type traceServer struct {
	tracepb.UnimplementedTraceServiceServer
	Sink
}

func (s traceServer) Export(ctx context.Context, req *tracepb.ExportTraceServiceRequest) (*tracepb.ExportTraceServiceResponse, error) {
	return s.Sink.ExportTraces(ctx, req)
}

type traceClient struct {
	client tracepb.TraceServiceClient
}

func (c traceClient) ExportTraces(ctx context.Context, req *tracepb.ExportTraceServiceRequest) (*tracepb.ExportTraceServiceResponse, error) {
	return c.client.Export(ctx, req)
}
