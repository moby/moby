package supervisor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/containerd/v2/cmd/containerd/server/config"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestConfigDropInsCannotOverrideRequiredSettings(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	dropInDir := t.TempDir()
	err := os.WriteFile(filepath.Join(dropInDir, "10-custom.toml"), []byte(`
version = 3
root = "/custom/root"

[grpc]
  address = "/custom/containerd.sock"

[plugins."io.containerd.gc.v1.scheduler"]
  pause_threshold = 0.04
`), 0o644)
	assert.NilError(t, err)

	r := &Daemon{
		config: config.Config{
			Version: 2,
			Root:    "/docker/containerd/root",
			State:   filepath.Join(stateDir, "daemon"),
			GRPC: config.GRPCConfig{ //nolint:staticcheck // Deprecated in config v4, but required for config v3.
				Address: defaultGRPCAddress(stateDir), //nolint:staticcheck // Deprecated in config v4, but required for config v3.
			},
		},
		stateDir:   stateDir,
		configFile: filepath.Join(stateDir, configFile),
		dropInDir:  dropInDir,
	}
	cfgFile, err := r.getContainerdConfig()
	assert.NilError(t, err)

	// Load the generated configuration the same way containerd does.
	var loaded config.Config
	err = config.LoadConfig(t.Context(), cfgFile, &loaded)
	assert.NilError(t, err)

	assert.Check(t, is.Equal(loaded.Root, r.config.Root))
	assert.Check(t, is.Equal(loaded.State, r.config.State))
	assert.Check(t, is.Equal(loaded.GRPC.Address, r.Address())) //nolint:staticcheck // Deprecated in config v4, but required for config v3.
	assert.Check(t, is.Contains(loaded.Plugins, "io.containerd.gc.v1.scheduler"))
}

func TestConfigDropInsWithRelativeStateDir(t *testing.T) {
	t.Chdir(t.TempDir())

	stateDir := filepath.Join("run", "containerd")
	assert.NilError(t, os.MkdirAll(stateDir, 0o700))

	r := &Daemon{
		config: config.Config{
			Version: 2,
			Root:    "/docker/containerd/root",
		},
		stateDir:   stateDir,
		configFile: filepath.Join(stateDir, configFile),
		dropInDir:  t.TempDir(),
	}
	cfgFile, err := r.getContainerdConfig()
	assert.NilError(t, err)

	var loaded config.Config
	err = config.LoadConfig(t.Context(), cfgFile, &loaded)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(loaded.Root, r.config.Root))
}

func TestConfigDropInsRejectImports(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	dropInDir := t.TempDir()
	dropInFile := filepath.Join(dropInDir, "10-custom.toml")
	err := os.WriteFile(dropInFile, []byte(`
version = 3
imports = ["nested.conf"]
`), 0o644)
	assert.NilError(t, err)

	r := &Daemon{
		stateDir:   stateDir,
		configFile: filepath.Join(stateDir, configFile),
		dropInDir:  dropInDir,
	}
	_, err = r.getContainerdConfig()
	assert.ErrorContains(t, err, "must not contain imports")
	assert.ErrorContains(t, err, dropInFile)
}
