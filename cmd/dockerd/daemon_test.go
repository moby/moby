package main

import (
	"testing"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/internal/testutil"
	"github.com/gotestyourself/gotestyourself/fs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultOptions(configFile string) *daemonOptions {
	opts := newDaemonOptions(&config.Config{})
	opts.flags = &pflag.FlagSet{}
	opts.InstallFlags(opts.flags)
	installConfigFlags(opts.daemonConfig, opts.flags)
	opts.flags.StringVar(&opts.configFile, "config-file", defaultDaemonConfigFile, "")
	opts.configFile = configFile
	return opts
}

func TestLoadDaemonCliConfigWithoutOverriding(t *testing.T) {
	opts := defaultOptions("")
	opts.Debug = true

	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)
	if !loadedConfig.Debug {
		t.Fatalf("expected debug to be copied from the common flags, got false")
	}
}

func TestLoadDaemonCliConfigWithTLS(t *testing.T) {
	opts := defaultOptions("")
	opts.TLSOptions.CAFile = "/tmp/ca.pem"
	opts.TLS = true

	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)
	assert.Equal(t, "/tmp/ca.pem", loadedConfig.CommonTLSOptions.CAFile)
}

func TestLoadDaemonCliConfigWithConflicts(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"labels": ["l3=foo"]}`))
	defer tempFile.Remove()
	configFile := tempFile.Path()

	opts := defaultOptions(configFile)
	flags := opts.flags

	assert.NoError(t, flags.Set("config-file", configFile))
	assert.NoError(t, flags.Set("label", "l1=bar"))
	assert.NoError(t, flags.Set("label", "l2=baz"))

	_, err := loadDaemonCliConfig(opts)
	testutil.ErrorContains(t, err, "as a flag and in the configuration file: labels")
}

func TestLoadDaemonCliWithConflictingLabels(t *testing.T) {
	opts := defaultOptions("")
	flags := opts.flags

	assert.NoError(t, flags.Set("label", "foo=bar"))
	assert.NoError(t, flags.Set("label", "foo=baz"))

	_, err := loadDaemonCliConfig(opts)
	assert.EqualError(t, err, "conflict labels for foo=baz and foo=bar")
}

func TestLoadDaemonCliWithDuplicateLabels(t *testing.T) {
	opts := defaultOptions("")
	flags := opts.flags

	assert.NoError(t, flags.Set("label", "foo=the-same"))
	assert.NoError(t, flags.Set("label", "foo=the-same"))

	_, err := loadDaemonCliConfig(opts)
	assert.NoError(t, err)
}

func TestLoadDaemonCliConfigWithTLSVerify(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"tlsverify": true}`))
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Path())
	opts.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)
	assert.Equal(t, loadedConfig.TLS, true)
}

func TestLoadDaemonCliConfigWithExplicitTLSVerifyFalse(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"tlsverify": false}`))
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Path())
	opts.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)
	assert.True(t, loadedConfig.TLS)
}

func TestLoadDaemonCliConfigWithoutTLSVerify(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{}`))
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Path())
	opts.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)
	assert.False(t, loadedConfig.TLS)
}

func TestLoadDaemonCliConfigWithLogLevel(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"log-level": "warn"}`))
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)
	assert.Equal(t, "warn", loadedConfig.LogLevel)
	assert.Equal(t, logrus.WarnLevel, logrus.GetLevel())
}

func TestLoadDaemonConfigWithEmbeddedOptions(t *testing.T) {
	content := `{"tlscacert": "/etc/certs/ca.pem", "log-driver": "syslog"}`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)
	assert.Equal(t, "/etc/certs/ca.pem", loadedConfig.CommonTLSOptions.CAFile)
	assert.Equal(t, "syslog", loadedConfig.LogConfig.Type)
}

func TestLoadDaemonConfigWithRegistryOptions(t *testing.T) {
	content := `{
		"allow-nondistributable-artifacts": ["allow-nondistributable-artifacts.com"],
		"registry-mirrors": ["https://mirrors.docker.com"],
		"insecure-registries": ["https://insecure.docker.com"]
	}`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)

	assert.Len(t, loadedConfig.AllowNondistributableArtifacts, 1)
	assert.Len(t, loadedConfig.Mirrors, 1)
	assert.Len(t, loadedConfig.InsecureRegistries, 1)
}
