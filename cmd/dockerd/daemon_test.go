package main

import (
	"testing"

	"github.com/Sirupsen/logrus"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/docker/pkg/testutil/tempfile"
	"github.com/go-check/check"
	"github.com/spf13/pflag"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func defaultOptions(configFile string) daemonOptions {
	opts := daemonOptions{
		daemonConfig: &daemon.Config{},
		flags:        &pflag.FlagSet{},
		common:       cliflags.NewCommonOptions(),
	}
	opts.common.InstallFlags(opts.flags)
	opts.daemonConfig.InstallFlags(opts.flags)
	opts.flags.StringVar(&opts.configFile, flagDaemonConfigFile, defaultDaemonConfigFile, "")
	opts.configFile = configFile
	return opts
}

func (s *DockerSuite) TestLoadDaemonCliConfigWithoutOverriding(c *check.C) {
	opts := defaultOptions("")
	opts.common.Debug = true

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)
	if !loadedConfig.Debug {
		c.Fatalf("expected debug to be copied from the common flags, got false")
	}
}

func (s *DockerSuite) TestLoadDaemonCliConfigWithTLS(c *check.C) {
	opts := defaultOptions("")
	opts.common.TLSOptions.CAFile = "/tmp/ca.pem"
	opts.common.TLS = true

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)
	assert.Equal(c, loadedConfig.CommonTLSOptions.CAFile, "/tmp/ca.pem")
}

func (s *DockerSuite) TestLoadDaemonCliConfigWithConflicts(c *check.C) {
	tempFile := tempfile.NewTempFile(c, "config", `{"labels": ["l3=foo"]}`)
	defer tempFile.Remove()
	configFile := tempFile.Name()

	opts := defaultOptions(configFile)
	flags := opts.flags

	assert.NilError(c, flags.Set(flagDaemonConfigFile, configFile))
	assert.NilError(c, flags.Set("label", "l1=bar"))
	assert.NilError(c, flags.Set("label", "l2=baz"))

	_, err := loadDaemonCliConfig(opts)
	assert.Error(c, err, "as a flag and in the configuration file: labels")
}

func (s *DockerSuite) TestLoadDaemonCliConfigWithTLSVerify(c *check.C) {
	tempFile := tempfile.NewTempFile(c, "config", `{"tlsverify": true}`)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	opts.common.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)
	assert.Equal(c, loadedConfig.TLS, true)
}

func (s *DockerSuite) TestLoadDaemonCliConfigWithExplicitTLSVerifyFalse(c *check.C) {
	tempFile := tempfile.NewTempFile(c, "config", `{"tlsverify": false}`)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	opts.common.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)
	assert.Equal(c, loadedConfig.TLS, true)
}

func (s *DockerSuite) TestLoadDaemonCliConfigWithoutTLSVerify(c *check.C) {
	tempFile := tempfile.NewTempFile(c, "config", `{}`)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	opts.common.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)
	assert.Equal(c, loadedConfig.TLS, false)
}

func (s *DockerSuite) TestLoadDaemonCliConfigWithLogLevel(c *check.C) {
	tempFile := tempfile.NewTempFile(c, "config", `{"log-level": "warn"}`)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)
	assert.Equal(c, loadedConfig.LogLevel, "warn")
	assert.Equal(c, logrus.GetLevel(), logrus.WarnLevel)
}

func (s *DockerSuite) TestLoadDaemonConfigWithEmbeddedOptions(c *check.C) {
	content := `{"tlscacert": "/etc/certs/ca.pem", "log-driver": "syslog"}`
	tempFile := tempfile.NewTempFile(c, "config", content)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)
	assert.Equal(c, loadedConfig.CommonTLSOptions.CAFile, "/etc/certs/ca.pem")
	assert.Equal(c, loadedConfig.LogConfig.Type, "syslog")
}

func (s *DockerSuite) TestLoadDaemonConfigWithRegistryOptions(c *check.C) {
	content := `{
		"registry-mirrors": ["https://mirrors.docker.com"],
		"insecure-registries": ["https://insecure.docker.com"]
	}`
	tempFile := tempfile.NewTempFile(c, "config", content)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(c, err)
	assert.NotNil(c, loadedConfig)

	assert.Equal(c, len(loadedConfig.Mirrors), 1)
	assert.Equal(c, len(loadedConfig.InsecureRegistries), 1)
}
