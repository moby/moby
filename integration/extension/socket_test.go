package extension

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/moby/moby/v2/integration/extension/testdata/greeter"
	greeterpb "github.com/moby/moby/v2/internal/extensions/example/greeter/v0/protogen"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// TestSocketExposedGRPCService launches an extension that serves the example
// greeter gRPC service and opts it into socket exposure (the service.grpc
// point), then calls the greeter with a plain gRPC client over the daemon's API
// socket -- the same docker.sock the REST API is served on. It proves an
// extension can publish its own gRPC service to external clients, forwarded by
// the daemon without the daemon knowing the service's proto.
func TestSocketExposedGRPCService(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "the extension binary must be on the daemon's host")

	ctx := testutil.StartSpan(baseContext, t)

	extDir := buildGreeterExtension(ctx, t)
	startArgs := []string{"--extension-dir", extDir}
	if testEnv.DaemonInfo.OSType == "linux" {
		startArgs = append(startArgs, "--iptables=false", "--ip6tables=false")
	}

	d := daemon.New(t)
	d.Start(t, startArgs...)
	defer d.Stop(t)

	// The daemon multiplexes gRPC and REST on the same socket, so a gRPC client
	// dialing docker.sock reaches the exposed extension service.
	conn, err := grpc.NewClient(d.Sock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NilError(t, err)
	defer conn.Close()

	resp, err := greeterpb.NewGreeterClient(conn).Greet(ctx, &greeterpb.HelloRequest{Name: "world"})
	assert.NilError(t, err)
	assert.Equal(t, resp.GetMessage(), "hello world")
}

// buildGreeterExtension compiles the greeter fixture into an extensions
// directory and returns the directory to pass to --extension-dir.
func buildGreeterExtension(ctx context.Context, t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", filepath.Join(dir, greeter.ID), "./testdata/greeter/cmd/greeter")
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, "build greeter extension: %s", out)
	return dir
}
