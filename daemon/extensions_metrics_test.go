package daemon

import (
	"context"
	"net"
	"testing"

	"github.com/docker/go-connections/sockets"
	"github.com/moby/extensions"
	metricsinkv0 "github.com/moby/moby/v2/extpoints/metricsink/v0"
	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gotest.tools/v3/assert"
)

func TestMetricSinksFlushToEveryProvider(t *testing.T) {
	t.Parallel()

	local := make(metricRequests, 1)
	remote := make(metricRequests, 1)

	socket := sockets.NewInmemSocket(t.Name(), 0)
	server := grpc.NewServer()
	metricsinkv0.ServerPoint.Register(server, remote)

	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(socket) }()

	t.Cleanup(func() {
		server.Stop()
		assert.NilError(t, <-serveDone)
	})

	conn, err := grpc.NewClient("passthrough:///extension",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return socket.DialContext(ctx, "", addr)
		}),
	)
	assert.NilError(t, err)
	t.Cleanup(func() { assert.NilError(t, conn.Close()) })

	resolver := sinkResolver{
		point: metricsinkv0.Point.ID(),
		providers: []extensions.ResolvedProvider{
			{Identity: extensions.ExtensionIdentity{ID: "org.example.local.v1"}, Impl: local},
			{Identity: extensions.ExtensionIdentity{ID: "org.example.remote.v1"}, Impl: metricsinkv0.ClientPoint.Provider(conn).Impl},
		},
	}

	mp, stop, err := startMetricSinks(t.Context(), resolver)
	assert.NilError(t, err)
	assert.Assert(t, mp != nil)

	counter, err := mp.Meter(t.Name()).Int64Counter("exported")
	assert.NilError(t, err)
	counter.Add(t.Context(), 3)
	stop(t.Context())

	for _, sink := range []metricRequests{local, remote} {
		assert.Equal(t, len(sink), 1)
		req := <-sink
		m := req.ResourceMetrics[0].ScopeMetrics[0].Metrics[0]
		assert.Equal(t, m.Name, "exported")
		assert.Equal(t, m.GetSum().DataPoints[0].GetAsInt(), int64(3))
	}

	// Shutting down the readers does not close the shared connection.
	_, err = collectormetricspb.NewMetricsServiceClient(conn).Export(t.Context(), &collectormetricspb.ExportMetricsServiceRequest{})
	assert.NilError(t, err)
}

type metricRequests chan *collectormetricspb.ExportMetricsServiceRequest

func (r metricRequests) ExportMetrics(ctx context.Context, req *collectormetricspb.ExportMetricsServiceRequest) (*collectormetricspb.ExportMetricsServiceResponse, error) {
	select {
	case r <- req:
		return &collectormetricspb.ExportMetricsServiceResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
