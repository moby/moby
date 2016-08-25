package main

import (
	"testing"

	"github.com/Sirupsen/logrus"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/docker/pkg/testutil/tempfile"
	"github.com/spf13/pflag"
)

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

func TestLoadDaemonCliConfigWithoutOverriding(t *testing.T) {
	opts := defaultOptions("")
	opts.common.Debug = true

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.NotNil(t, loadedConfig)
	if !loadedConfig.Debug {
		t.Fatalf("expected debug to be copied from the common flags, got false")
	}
}

func TestLoadDaemonCliConfigWithTLS(t *testing.T) {
	opts := defaultOptions("")
	opts.common.TLSOptions.CAFile = "/tmp/ca.pem"
	opts.common.TLS = true

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.NotNil(t, loadedConfig)
	assert.Equal(t, loadedConfig.CommonTLSOptions.CAFile, "/tmp/ca.pem")
}

func TestLoadDaemonCliConfigWithConflicts(t *testing.T) {
	tempFile := tempfile.NewTempFile(t, "config", `{"labels": ["l3=foo"]}`)
	defer tempFile.Remove()
	configFile := tempFile.Name()

	opts := defaultOptions(configFile)
	flags := opts.flags

	assert.NilError(t, flags.Set(flagDaemonConfigFile, configFile))
	assert.NilError(t, flags.Set("label", "l1=bar"))
	assert.NilError(t, flags.Set("label", "l2=baz"))

	_, err := loadDaemonCliConfig(opts)
	assert.Error(t, err, "as a flag and in the configuration file: labels")
}

func TestLoadDaemonCliConfigWithTLSVerify(t *testing.T) {
	tempFile := tempfile.NewTempFile(t, "config", `{"tlsverify": true}`)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	opts.common.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.NotNil(t, loadedConfig)
	assert.Equal(t, loadedConfig.TLS, true)
}

func TestLoadDaemonCliConfigWithExplicitTLSVerifyFalse(t *testing.T) {
	tempFile := tempfile.NewTempFile(t, "config", `{"tlsverify": false}`)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	opts.common.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.NotNil(t, loadedConfig)
	assert.Equal(t, loadedConfig.TLS, true)
}

func TestLoadDaemonCliConfigWithoutTLSVerify(t *testing.T) {
	tempFile := tempfile.NewTempFile(t, "config", `{}`)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	opts.common.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.NotNil(t, loadedConfig)
	assert.Equal(t, loadedConfig.TLS, false)
}

func TestLoadDaemonCliConfigWithLogLevel(t *testing.T) {
	tempFile := tempfile.NewTempFile(t, "config", `{"log-level": "warn"}`)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.NotNil(t, loadedConfig)
	assert.Equal(t, loadedConfig.LogLevel, "warn")
	assert.Equal(t, logrus.GetLevel(), logrus.WarnLevel)
}

func TestLoadDaemonConfigWithEmbeddedOptions(t *testing.T) {
	content := `{"tlscacert": "/etc/certs/ca.pem", "log-driver": "syslog"}`
	tempFile := tempfile.NewTempFile(t, "config", content)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.NotNil(t, loadedConfig)
	assert.Equal(t, loadedConfig.CommonTLSOptions.CAFile, "/etc/certs/ca.pem")
	assert.Equal(t, loadedConfig.LogConfig.Type, "syslog")
}

func TestLoadDaemonConfigWithRegistryOptions(t *testing.T) {
	content := `{
		"registry-mirrors": ["https://mirrors.docker.com"],
		"insecure-registries": ["https://insecure.docker.com"]
	}`
	tempFile := tempfile.NewTempFile(t, "config", content)
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Name())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.NotNil(t, loadedConfig)

	assert.Equal(t, len(loadedConfig.Mirrors), 1)
	assert.Equal(t, len(loadedConfig.InsecureRegistries), 1)
}
