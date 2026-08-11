package host_test

import (
	"context"
	"net"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	servicegrpcv0 "github.com/moby/moby/v2/extpoints/servicegrpc/v0"
	"github.com/moby/moby/v2/integration/extension/testdata/greeter"
	"github.com/moby/moby/v2/internal/extensions"
	"github.com/moby/moby/v2/internal/extensions/clientpoint"
	greeterpb "github.com/moby/moby/v2/internal/extensions/example/greeter/v0/protogen"
	"github.com/moby/moby/v2/internal/extensions/grpcproxy"
	"github.com/moby/moby/v2/internal/extensions/host"
	echov1 "github.com/moby/moby/v2/internal/extensions/internal/launcher/echo/v1"
	echopb "github.com/moby/moby/v2/internal/extensions/internal/launcher/echo/v1/protogen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gotest.tools/v3/assert"
)

// TestSocketExposure verifies an out-of-process service is reachable by name
// through a proxy without importing its proto.
func TestSocketExposure(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and launches a helper binary")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, greeter.ID)
	build := exec.Command("go", "build", "-o", bin, "github.com/moby/moby/v2/integration/extension/testdata/greeter/cmd/greeter")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build greeter extension: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	h, err := host.New(ctx, host.Options{
		RuntimeDir:       t.TempDir(),
		Dirs:             []string{dir},
		ExposeOnlyPoints: []extensions.PointID{servicegrpcv0.Point.ID()},
	})
	assert.NilError(t, err)
	defer func() { assert.NilError(t, h.Shutdown(context.Background())) }()

	routes := map[string]grpc.ClientConnInterface{}
	for ext, names := range h.ServicesForPoint(servicegrpcv0.Point.ID()) {
		conn, ok := h.Conn(ext)
		assert.Check(t, ok, "no connection for extension %q", ext)
		for _, name := range names {
			routes[name] = conn
		}
	}
	assert.Check(t, routes["org.mobyproject.extension.example.greeter.v0.Greeter"] != nil)

	sock := filepath.Join(t.TempDir(), "api.sock")
	lis, err := net.Listen("unix", sock)
	assert.NilError(t, err)
	proxy := grpcproxy.New(routes)
	go proxy.Serve(lis)
	defer proxy.Stop()

	conn, err := grpc.NewClient("unix:"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NilError(t, err)
	defer conn.Close()

	resp, err := greeterpb.NewGreeterClient(conn).Greet(ctx, &greeterpb.HelloRequest{Name: "world"})
	assert.NilError(t, err)
	assert.Equal(t, resp.GetMessage(), "hello world")
}

// TestHookOnlyServicesAreNotSocketExposed verifies hook services stay on the
// private extension socket without service.grpc opt-in.
func TestHookOnlyServicesAreNotSocketExposed(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and launches a helper binary")
	}

	dir := t.TempDir()
	const id = "org.example.exthook.v1"
	bin := filepath.Join(dir, id)
	build := exec.Command("go", "build", "-o", bin, "github.com/moby/moby/v2/internal/extensions/internal/launcher/testdata/exthook")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build exthook extension: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	h, err := host.New(ctx, host.Options{
		RuntimeDir:      t.TempDir(),
		Dirs:            []string{dir},
		ClientProviders: []clientpoint.Registration{echopb.ClientPoint},
	})
	assert.NilError(t, err)
	defer func() { assert.NilError(t, h.Shutdown(context.Background())) }()

	assert.Check(t, h.ServicesForPoint(servicegrpcv0.Point.ID())[id] == nil)
	assert.DeepEqual(t, h.ServicesForPoint(echov1.Point.ID())[id], []string{"moby.extensions.internal.launcher.echo.v1.Echo"})

	conn, ok := h.Conn(id)
	assert.Check(t, ok)
	client := echopb.NewEchoClient(conn)
	resp, err := client.Echo(ctx, &echopb.EchoRequest{Message: "private"})
	assert.NilError(t, err)
	assert.Equal(t, resp.GetMessage(), "private")
}

// TestInProcessServiceExposure verifies an in-process service is registered
// directly on the host gRPC server.
func TestInProcessServiceExposure(t *testing.T) {
	ctx := context.Background()
	h, err := host.New(ctx, host.Options{
		RuntimeDir: t.TempDir(),
		Extensions: []extensions.Extension{greeter.Extension},
	})
	assert.NilError(t, err)
	defer func() { assert.NilError(t, h.Shutdown(context.Background())) }()

	services, err := servicegrpcv0.Collect(h)
	assert.NilError(t, err)
	srv := grpc.NewServer()
	for _, svc := range services {
		srv.RegisterService(svc.Desc, svc.Impl)
	}

	sock := filepath.Join(t.TempDir(), "api.sock")
	lis, err := net.Listen("unix", sock)
	assert.NilError(t, err)
	go srv.Serve(lis)
	defer srv.Stop()

	conn, err := grpc.NewClient("unix:"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NilError(t, err)
	defer conn.Close()

	resp, err := greeterpb.NewGreeterClient(conn).Greet(ctx, &greeterpb.HelloRequest{Name: "world"})
	assert.NilError(t, err)
	assert.Equal(t, resp.GetMessage(), "hello world")
}
