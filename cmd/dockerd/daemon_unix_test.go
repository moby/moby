// +build !windows

package main

import (
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/docker/pkg/testutil/tempfile"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestLoadDaemonCliConfigWithDaemonFlags(c *check.C) {
	content := `{"log-opts": {"max-size": "1k"}}`
	tempFile := tempfile.NewTempFile(c, "config", content)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	opts.common.Debug = true
	opts.common.LogLevel = "info"
	assert.NilError(c, opts.flags.Set("selinux-enabled", "true"))

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)

	assert.Equal(c, loadedConfig.Debug, true)
	assert.Equal(c, loadedConfig.LogLevel, "info")
	assert.Equal(c, loadedConfig.EnableSelinuxSupport, true)
	assert.Equal(c, loadedConfig.LogConfig.Type, "json-file")
	assert.Equal(c, loadedConfig.LogConfig.Config["max-size"], "1k")
}

func (s *DockerSuite) TestLoadDaemonConfigWithNetwork(c *check.C) {
	content := `{"bip": "127.0.0.2", "ip": "127.0.0.1"}`
	tempFile := tempfile.NewTempFile(c, "config", content)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)

	assert.Equal(c, loadedConfig.IP, "127.0.0.2")
	assert.Equal(c, loadedConfig.DefaultIP.String(), "127.0.0.1")
}

func (s *DockerSuite) TestLoadDaemonConfigWithMapOptions(c *check.C) {
	content := `{
		"cluster-store-opts": {"kv.cacertfile": "/var/lib/docker/discovery_certs/ca.pem"},
		"log-opts": {"tag": "test"}
}`
	tempFile := tempfile.NewTempFile(c, "config", content)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)
	assert.NotNil(c, loadedConfig.ClusterOpts)

	expectedPath := "/var/lib/docker/discovery_certs/ca.pem"
	assert.Equal(c, loadedConfig.ClusterOpts["kv.cacertfile"], expectedPath)
	assert.NotNil(c, loadedConfig.LogConfig.Config)
	assert.Equal(c, loadedConfig.LogConfig.Config["tag"], "test")
}

func (s *DockerSuite) TestLoadDaemonConfigWithTrueDefaultValues(c *check.C) {
	content := `{ "userland-proxy": false }`
	tempFile := tempfile.NewTempFile(c, "config", content)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)
	assert.NotNil(c, loadedConfig.ClusterOpts)

	assert.Equal(c, loadedConfig.EnableUserlandProxy, false)

	// make sure reloading doesn't generate configuration
	// conflicts after normalizing boolean values.
	reload := func(reloadedConfig *daemon.Config) {
		assert.Equal(c, reloadedConfig.EnableUserlandProxy, false)
	}
	assert.NilError(c, daemon.ReloadConfiguration(opts.configFile, opts.flags, reload))
}

func (s *DockerSuite) TestLoadDaemonConfigWithTrueDefaultValuesLeaveDefaults(c *check.C) {
	tempFile := tempfile.NewTempFile(c, "config", `{}`)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)
	assert.NotNil(c, loadedConfig.ClusterOpts)

	assert.Equal(c, loadedConfig.EnableUserlandProxy, true)
}

func (s *DockerSuite) TestLoadDaemonConfigWithLegacyRegistryOptions(c *check.C) {
	content := `{"disable-legacy-registry": true}`
	tempFile := tempfile.NewTempFile(c, "config", content)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)
	assert.Equal(c, loadedConfig.V2Only, true)
}
