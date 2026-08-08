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

// TestSocketExposure verifies socket exposure end to end without the daemon: it
// launches the greeter extension out of process, replicates what the daemon does
// (ask the service.grpc point which gRPC services to publish, build a proxy from
// each service name to the extension's connection), serves the proxy, and calls
// the greeter with a plain gRPC client.
//
// The daemon never imports the greeter; it forwards the bytes by service name.
// Only the docker.sock HTTP/gRPC multiplexing, which is buildkit's existing
// code, is left to the integration test.
func TestSocketExposure(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and launches a helper binary")
	}

	// Build the greeter extension binary (named after its id).
	dir := t.TempDir()
	bin := filepath.Join(dir, greeter.ID)
	build := exec.Command("go", "build", "-o", bin, "github.com/moby/moby/v2/integration/extension/testdata/greeter/cmd/greeter")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build greeter extension: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Launch the extension out of process. service.grpc is not a wire point, so
	// it needs no ClientProviders; listing it as expose-only lets the host accept
	// an extension that provides it. The extension reports the gRPC service names
	// it exposes in its declaration.
	h, err := host.New(ctx, host.Options{
		RuntimeDir:       t.TempDir(),
		Dirs:             []string{dir},
		ExposeOnlyPoints: []extensions.PointID{servicegrpcv0.Point.ID()},
	})
	assert.NilError(t, err)
	defer func() { assert.NilError(t, h.Shutdown(context.Background())) }()

	// Build the proxy routes the way the daemon does for out-of-process
	// extensions: each exposed service name to the connection of the extension
	// that serves it.
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

	// External gRPC client over the socket -- the daemon never saw the proto.
	conn, err := grpc.NewClient("unix:"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NilError(t, err)
	defer conn.Close()

	resp, err := greeterpb.NewGreeterClient(conn).Greet(ctx, &greeterpb.HelloRequest{Name: "world"})
	assert.NilError(t, err)
	assert.Equal(t, resp.GetMessage(), "hello world")
}

// TestHookOnlyServicesAreNotSocketExposed verifies the public-socket boundary:
// an out-of-process extension's provider RPCs are served on its private
// per-extension socket, but the daemon should only publish services registered
// by the service.grpc point. A hook-only extension therefore has callable
// private services and no public service.grpc inventory.
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

// TestInProcessServiceExposure verifies socket exposure works the same for an
// in-process extension: the *same* greeter extension, registered in-process,
// injects its gRPC service through the service.grpc point, and the daemon serves
// it directly on its own gRPC server (no proxy). It uses a plain grpc.Server in
// place of the daemon's.
func TestInProcessServiceExposure(t *testing.T) {
	ctx := context.Background()
	h, err := host.New(ctx, host.Options{
		RuntimeDir: t.TempDir(),
		Extensions: []extensions.Extension{greeter.Extension},
	})
	assert.NilError(t, err)
	defer func() { assert.NilError(t, h.Shutdown(context.Background())) }()

	// The daemon collects in-process extensions' services and registers them on
	// its own gRPC server.
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
