package logging

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/testutil/fixtures/plugin"
	"github.com/moby/locker"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
)

var pluginBuildLock = locker.New()

func ensurePlugin(t *testing.T, name string) string {
	pluginBuildLock.Lock(name)
	defer pluginBuildLock.Unlock(name)

	installPath := filepath.Join(os.Getenv("GOPATH"), "bin", name)
	if _, err := os.Stat(installPath); err == nil {
		return installPath
	}

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, "build", "-o", installPath, "./"+filepath.Join("cmd", name))
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GO111MODULE=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(errors.Wrapf(err, "error building basic plugin bin: %s", string(out)))
	}

	return installPath
}

func withSockPath(name string) func(*plugin.Config) {
	return func(cfg *plugin.Config) {
		cfg.Interface.Socket = name
	}
}

func createPlugin(ctx context.Context, t *testing.T, client plugin.CreateClient, alias, bin string, opts ...plugin.CreateOpt) {
	t.Helper()

	pluginBin := ensurePlugin(t, bin)

	opts = append(opts, withSockPath("plugin.sock"))
	opts = append(opts, plugin.WithBinary(pluginBin))

	err := plugin.Create(ctx, client, alias, opts...)
	assert.NilError(t, err)
}

func asLogDriver(cfg *plugin.Config) {
	cfg.Interface.Types = []types.PluginInterfaceType{
		{Capability: "logdriver", Prefix: "docker", Version: "1.0"},
	}
}
