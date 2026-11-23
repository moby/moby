package daemon

import (
	"context"
	"net"
	"testing"

	"github.com/docker/go-connections/sockets"
	"github.com/moby/extensions"
	tracesinkv0 "github.com/moby/moby/v2/extpoints/tracesink/v0"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gotest.tools/v3/assert"
)

func TestTraceSinksDrainToEveryProvider(t *testing.T) {
	t.Parallel()

	local := make(traceRequests, 1)
	remote := make(traceRequests, 2)

	socket := sockets.NewInmemSocket(t.Name(), 0)
	server := grpc.NewServer()
	tracesinkv0.ServerPoint.Register(server, remote)

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
		point: tracesinkv0.Point.ID(),
		providers: []extensions.ResolvedProvider{
			{Identity: extensions.ExtensionIdentity{ID: "org.example.local.v1"}, Impl: local},
			{Identity: extensions.ExtensionIdentity{ID: "org.example.remote.v1"}, Impl: tracesinkv0.ClientPoint.Provider(conn).Impl},
		},
	}
	tp := sdktrace.NewTracerProvider()

	t.Cleanup(func() { assert.NilError(t, tp.Shutdown(context.WithoutCancel(t.Context()))) })

	stop, err := startTraceSinks(t.Context(), tp, resolver)
	assert.NilError(t, err)

	_, span := tp.Tracer(t.Name()).Start(t.Context(), "export me")
	span.End()
	stop(t.Context())

	for _, sink := range []traceRequests{local, remote} {
		assert.Equal(t, len(sink), 1)
		req := <-sink
		assert.Equal(t, req.ResourceSpans[0].ScopeSpans[0].Spans[0].Name, "export me")
	}

	// Shutting down the exporters does not close the shared connection.
	_, err = collectortracepb.NewTraceServiceClient(conn).Export(t.Context(), &collectortracepb.ExportTraceServiceRequest{})
	assert.NilError(t, err)
}

type traceRequests chan *collectortracepb.ExportTraceServiceRequest

func (r traceRequests) ExportTraces(ctx context.Context, req *collectortracepb.ExportTraceServiceRequest) (*collectortracepb.ExportTraceServiceResponse, error) {
	select {
	case r <- req:
		return &collectortracepb.ExportTraceServiceResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// sinkResolver resolves providers for a single point.
type sinkResolver struct {
	extensions.Resolver
	point     extensions.PointID
	providers []extensions.ResolvedProvider
}

func (r sinkResolver) Providers(point extensions.PointID) []extensions.ResolvedProvider {
	if point == r.point {
		return r.providers
	}

	return nil
}
