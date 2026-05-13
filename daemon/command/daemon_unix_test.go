//go:build unix

package command

import (
	"testing"

	"github.com/moby/moby/v2/daemon/config"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"
)

func TestLoadDaemonConfigWithDaemonFlags(t *testing.T) {
	content := `{"log-opts": {"max-size": "1k"}}`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))

	opts := defaultOptions(t, tempFile.Path())
	opts.Debug = true
	opts.daemonConfig.DaemonLogConfig.LogLevel = "warn"
	assert.Check(t, opts.flags.Set("selinux-enabled", "true"))

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)

	assert.Check(t, is.Equal(loadedConfig.Debug, true))
	assert.Check(t, is.Equal(loadedConfig.DaemonLogConfig.LogLevel, "warn"))
	assert.Check(t, is.Equal(loadedConfig.EnableSelinuxSupport, true))
	assert.Check(t, is.Equal(loadedConfig.LogConfig.Type, "json-file"))
	assert.Check(t, is.Equal(loadedConfig.LogConfig.Config["max-size"], "1k"))
}

func TestLoadDaemonConfigWithNetwork(t *testing.T) {
	content := `{"bip": "127.0.0.2/8", "bip6": "fd98:e5f2:e637::1/64", "ip": "127.0.0.1"}`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))

	opts := defaultOptions(t, tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)

	assert.Check(t, is.Equal(loadedConfig.IP, "127.0.0.2/8"))
	assert.Check(t, is.Equal(loadedConfig.IP6, "fd98:e5f2:e637::1/64"))
	assert.Check(t, is.Equal(loadedConfig.DefaultIP.String(), "127.0.0.1"))
}

func TestLoadDaemonConfigWithMapOptions(t *testing.T) {
	content := `{"log-opts": {"tag": "test"}}`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))

	opts := defaultOptions(t, tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)

	assert.Check(t, loadedConfig.LogConfig.Config != nil)
	assert.Check(t, is.Equal("test", loadedConfig.LogConfig.Config["tag"]))
}

func TestLoadDaemonConfigWithTrueDefaultValues(t *testing.T) {
	content := `{ "icc": false }`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))

	opts := defaultOptions(t, tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)

	assert.Check(t, is.Equal(loadedConfig.InterContainerCommunication, false))

	// make sure reloading doesn't generate configuration
	// conflicts after normalizing boolean values.
	assert.Check(t, config.Reload(opts.configFile, opts.flags, func(reloadedConfig *config.Config) {
		assert.Check(t, is.Equal(reloadedConfig.InterContainerCommunication, false))
	}))
}

func TestLoadDaemonConfigWithTrueDefaultValuesLeaveDefaults(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{}`))

	opts := defaultOptions(t, tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)

	assert.Check(t, is.Equal(loadedConfig.InterContainerCommunication, true))
}
