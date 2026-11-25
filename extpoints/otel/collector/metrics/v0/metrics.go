// Package metricsv0 describes the otel metrics extension point used for
// tying an otel collector to the moby daemon.
package metricsv0

import (
	"context"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/grpc"
)

type Exporter interface {
	Export(ctx context.Context, rm *metricdata.ResourceMetrics) error
}

var Point = extensions.DefinePoint[Exporter]("org.mobyproject.extension.otel.collector.metrics.v0")

var ClientPoint = clientpoint.Registration{
	Point: Point.ID(),
	Provider: func(conn grpc.ClientConnInterface) extensions.Provider {
		ctx := context.TODO()
		impl, err := newExporter(ctx, conn)
		if err != nil {
			log.G(ctx).WithError(err).Error("error setting up telemetry metric extension client point")
			impl = noopExporter{}
		}
		return Point.Provide(impl)
	},
}

func newExporter(ctx context.Context, conn grpc.ClientConnInterface) (Exporter, error) {
	// The public API for otlpmetricgrpc does not have anyway to use the interface
	// at the current moment so we need to cast this to the underlying type.
	// Unlike otlptrace, it also doesn't have any generic interface that works for both
	// the http and grpc uploads and the metrics transformation from the public type to
	// the protobuf definition is in an internal package.
	c, ok := conn.(*grpc.ClientConn)
	if !ok {
		return nil, cerrdefs.ErrInternal
	}
	return otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(c))
}

type noopExporter struct{}

func (noopExporter) Export(ctx context.Context, rm *metricdata.ResourceMetrics) error {
	return nil
}
