package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moby/moby/v2/internal/extensions"
	"github.com/moby/moby/v2/internal/extensions/sdk/sdkpb"
	"github.com/moby/moby/v2/internal/extensions/serverpoint"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestRegisterBuildsDeclaration(t *testing.T) {
	srv := NewServer()
	registered := false
	point := serverpoint.Registration{
		Point:    "org.example.point.v1",
		Register: func(grpc.ServiceRegistrar, any) { registered = true },
	}
	ext := extensions.New(extensions.Declaration{
		ID:           "org.example.extension.v1",
		Providers:    []extensions.Provider{{Point: "org.example.point.v1", Impl: struct{}{}}},
		Dependencies: []extensions.Dependency{{Extension: "org.example.dependency"}},
		Conflicts:    []extensions.ExtensionID{"org.example.conflict"},
	})

	err := srv.Register(ext, point)
	assert.NilError(t, err)
	assert.Check(t, registered)

	d := srv.declaration
	assert.Equal(t, d.GetId(), "org.example.extension.v1")
	assert.Check(t, is.Len(d.GetProviders(), 1))
	assert.Equal(t, d.GetProviders()[0].GetId(), "org.example.point.v1")
	assert.Check(t, is.Len(d.GetDependencies(), 1))
	assert.Equal(t, d.GetDependencies()[0].GetExtension(), "org.example.dependency")
	assert.DeepEqual(t, d.GetConflicts(), []string{"org.example.conflict"})
}

// TestRegisterRecordsServedServices locks down that the gRPC service names each
// provider registers are recorded by provider point. Describe reports that
// inventory to the daemon, which decides which point's services to publish on
// the API socket. A provider that registers no service records an empty list.
func TestRegisterRecordsServedServices(t *testing.T) {
	desc := &grpc.ServiceDesc{ServiceName: "org.example.point.v1.Thing", HandlerType: (*any)(nil)}
	served := serverpoint.Registration{
		Point:    "org.example.point.v1",
		Register: func(r grpc.ServiceRegistrar, impl any) { r.RegisterService(desc, impl) },
	}
	srv := NewServer()
	assert.NilError(t, srv.Register(extensions.New(extensions.Declaration{
		ID:        "org.example.extension.v1",
		Providers: []extensions.Provider{{Point: "org.example.point.v1", Impl: struct{}{}}},
	}), served))
	assert.Check(t, is.Len(srv.declaration.GetExposedServices(), 0))
	assert.Check(t, is.Len(srv.declaration.GetProviderServices(), 1))
	assert.Equal(t, srv.declaration.GetProviderServices()[0].GetPoint(), "org.example.point.v1")
	assert.DeepEqual(t, srv.declaration.GetProviderServices()[0].GetServices(), []string{"org.example.point.v1.Thing"})

	noService := serverpoint.Registration{
		Point:    "org.example.point.v1",
		Register: func(grpc.ServiceRegistrar, any) {},
	}
	plainSrv := NewServer()
	assert.NilError(t, plainSrv.Register(extensions.New(extensions.Declaration{
		ID:        "org.example.extension.v1",
		Providers: []extensions.Provider{{Point: "org.example.point.v1", Impl: struct{}{}}},
	}), noService))
	assert.Check(t, is.Len(plainSrv.declaration.GetProviderServices(), 1))
	assert.Check(t, is.Len(plainSrv.declaration.GetProviderServices()[0].GetServices(), 0))
}

func TestRegisterRejectsUnknownPoint(t *testing.T) {
	srv := NewServer()
	ext := extensions.New(extensions.Declaration{
		ID:        "org.example.extension.v1",
		Providers: []extensions.Provider{{Point: "org.example.point.v1", Impl: struct{}{}}},
	})

	err := srv.Register(ext)
	assert.ErrorContains(t, err, "no server registration for point")
}

func TestListenRejectsUnsupportedProtocol(t *testing.T) {
	srv := NewServer()
	err := srv.ListenWithIO(context.Background(), strings.NewReader(`{"endpoint":"/tmp/x.sock","protocolVersion":999}`), io.Discard)
	assert.ErrorContains(t, err, "unsupported extension protocol version")
}

// TestListenDeliversConfig locks down that the config from the startup handshake
// reaches the extension's Init -- an out-of-process extension is configured by
// id just like an in-process one. Init is deferred until the daemon calls the
// Initialize RPC, so the test drives that RPC.
func TestListenDeliversConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got extensions.Config
	ext := extensions.New(extensions.Declaration{
		ID: "org.example.extension.v1",
		Init: func(_ context.Context, cfg extensions.Config, _ extensions.Resolver) error {
			got = cfg
			return nil
		},
	})
	srv := NewServer()
	assert.NilError(t, srv.Register(ext))

	endpoint := filepath.Join(t.TempDir(), "x.sock")
	in, err := json.Marshal(StartupConfig{
		Endpoint:        endpoint,
		ProtocolVersion: ProtocolVersion,
		Config:          extensions.Config{"plugin_path": "/opt/nri", "enabled": true},
	})
	assert.NilError(t, err)

	// Serve in the background; the readiness ack on stdout signals it is listening.
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- srv.ListenWithIO(ctx, bytes.NewReader(in), pw) }()
	_, err = io.ReadFull(pr, make([]byte, len(ReadinessAck)))
	assert.NilError(t, err)

	conn, err := grpc.NewClient("unix:"+endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(c context.Context, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(c, "unix", endpoint)
		}))
	assert.NilError(t, err)
	defer conn.Close()

	_, err = sdkpb.NewExtensionClient(conn).Initialize(ctx, &sdkpb.InitializeRequest{})
	assert.NilError(t, err)
	assert.Equal(t, got["plugin_path"], "/opt/nri")
	assert.Equal(t, got["enabled"], true)

	cancel()
	<-done
}
