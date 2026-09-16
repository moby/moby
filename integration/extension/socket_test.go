package extension

import (
	"context"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	extensionclient "github.com/moby/extensions/client"
	greeterv0 "github.com/moby/extensions/example/greeter/v0"
	greeterpb "github.com/moby/extensions/example/greeter/v0/protogen"
	"github.com/moby/moby/v2/integration/extension/testdata/greeter"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

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
	defer func() {
		if runtime.GOOS == "windows" {
			assert.NilError(t, d.Kill())
			return
		}
		d.Stop(t)
	}()

	engine := d.NewClientT(t)
	extensions, err := extensionclient.New(engine, extensionclient.WithGRPCPoint(greeterpb.ClientPoint))
	assert.NilError(t, err)
	defer func() { assert.NilError(t, extensions.Close()) }()

	greeter, err := extensionclient.Resolve(extensions, greeterv0.Point)
	assert.NilError(t, err)
	resp, err := greeter.Greet(ctx, &greeterv0.HelloRequest{Name: "world"})
	assert.NilError(t, err)
	assert.Equal(t, resp.Message, "hello world")
}

func buildGreeterExtension(ctx context.Context, t *testing.T) string {
	t.Helper()
	dir := testutil.TempDir(t)
	// The extension directory is read by the daemon, which may run as an
	// unprivileged user in rootless mode.
	assert.NilError(t, os.Chmod(dir, 0o755))
	name := greeter.ID
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(dir, name)
	cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", bin, "./testdata/greeter/cmd/greeter")
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, "build greeter extension: %s", out)

	if testEnv.IsRootless() {
		// RootlessKit maps unprivilegeduser to root in the daemon's user
		// namespace. Use that ownership so discovery trusts both entries.
		rootlessUser, err := user.Lookup("unprivilegeduser")
		assert.NilError(t, err)
		uid, err := strconv.Atoi(rootlessUser.Uid)
		assert.NilError(t, err)
		gid, err := strconv.Atoi(rootlessUser.Gid)
		assert.NilError(t, err)
		assert.NilError(t, os.Chown(dir, uid, gid))
		assert.NilError(t, os.Chown(bin, uid, gid))
	}
	return dir
}
