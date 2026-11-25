// Package tracev0 describes the otel trace extension point used for
// tying an otel collector to the moby daemon.
package tracev0

import (
	"context"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
)

type Client interface {
	UploadTraces(ctx context.Context, protoSpans []*tracev1.ResourceSpans) error
}

var Point = extensions.DefinePoint[Client]("org.mobyproject.extension.otel.collector.trace.v0")

var ClientPoint = clientpoint.Registration{
	Point: Point.ID(),
	Provider: func(conn grpc.ClientConnInterface) extensions.Provider {
		impl := newClientProvider(conn)
		return Point.Provide(impl)
	},
}

type clientProvider struct {
	tracesClient tracepb.TraceServiceClient
}

func newClientProvider(conn grpc.ClientConnInterface) Client {
	return &clientProvider{
		tracesClient: tracepb.NewTraceServiceClient(conn),
	}
}

func (s *clientProvider) UploadTraces(ctx context.Context, protoSpans []*tracev1.ResourceSpans) error {
	in := &tracepb.ExportTraceServiceRequest{ResourceSpans: protoSpans}
	_, err := s.tracesClient.Export(ctx, in)
	return err
}
