// Package metricsinkv0 defines a point for receiving dockerd metric batches.
package metricsinkv0

import (
	"context"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/extensions/serverpoint"
	metricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/grpc"
)

// Sink receives batches of daemon metrics using the OTLP metrics protocol.
// Implementations must honor context cancellation and must not record
// measurements on the daemon's meter provider while processing exports.
type Sink interface {
	ExportMetrics(context.Context, *metricspb.ExportMetricsServiceRequest) (*metricspb.ExportMetricsServiceResponse, error)
}

// Point allows multiple metric sinks.
var Point = extensions.DefinePoint[Sink]("org.mobyproject.extension.telemetry.metricsink.v0")

// ServerPoint serves the standard OTLP metrics service on the extension server.
var ServerPoint = serverpoint.Registration{
	Point: Point.ID(),
	Register: func(r grpc.ServiceRegistrar, impl any) {
		metricspb.RegisterMetricsServiceServer(r, metricsServer{Sink: impl.(Sink)})
	},
}

// ClientPoint uses the host-owned connection to the extension.
var ClientPoint = clientpoint.Registration{
	Point: Point.ID(),
	Provider: func(conn grpc.ClientConnInterface) extensions.Provider {
		return Point.Provide(metricsClient{client: metricspb.NewMetricsServiceClient(conn)})
	},
}

type metricsServer struct {
	metricspb.UnimplementedMetricsServiceServer
	Sink
}

func (s metricsServer) Export(ctx context.Context, req *metricspb.ExportMetricsServiceRequest) (*metricspb.ExportMetricsServiceResponse, error) {
	return s.Sink.ExportMetrics(ctx, req)
}

type metricsClient struct {
	client metricspb.MetricsServiceClient
}

func (c metricsClient) ExportMetrics(ctx context.Context, req *metricspb.ExportMetricsServiceRequest) (*metricspb.ExportMetricsServiceResponse, error) {
	return c.client.Export(ctx, req)
}
