package main

import (
	"testing"

	"github.com/docker/docker/daemon/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"
)

func defaultOptions(t *testing.T, configFile string) *daemonOptions {
	cfg, err := config.New()
	assert.NilError(t, err)
	opts := newDaemonOptions(cfg)
	opts.flags = &pflag.FlagSet{}
	opts.installFlags(opts.flags)
	err = installConfigFlags(opts.daemonConfig, opts.flags)
	assert.NilError(t, err)
	defaultDaemonConfigFile, err := getDefaultDaemonConfigFile()
	assert.NilError(t, err)
	opts.flags.StringVar(&opts.configFile, "config-file", defaultDaemonConfigFile, "")
	opts.configFile = configFile
	err = opts.flags.Parse([]string{})
	assert.NilError(t, err)
	return opts
}

func TestLoadDaemonCliConfigWithoutOverriding(t *testing.T) {
	opts := defaultOptions(t, "")
	opts.Debug = true

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	if !loadedConfig.Debug {
		t.Fatalf("expected debug to be copied from the common flags, got false")
	}
}

func TestLoadDaemonCliConfigWithTLS(t *testing.T) {
	opts := defaultOptions(t, "")
	opts.TLSOptions.CAFile = "/tmp/ca.pem"
	opts.TLS = true

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	assert.Check(t, is.Equal("/tmp/ca.pem", loadedConfig.CommonTLSOptions.CAFile))
}

func TestLoadDaemonCliConfigWithConflicts(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"labels": ["l3=foo"]}`))
	defer tempFile.Remove()
	configFile := tempFile.Path()

	opts := defaultOptions(t, configFile)
	flags := opts.flags

	assert.Check(t, flags.Set("config-file", configFile))
	assert.Check(t, flags.Set("label", "l1=bar"))
	assert.Check(t, flags.Set("label", "l2=baz"))

	_, err := loadDaemonCliConfig(opts)
	assert.Check(t, is.ErrorContains(err, "as a flag and in the configuration file: labels"))
}

func TestLoadDaemonCliWithConflictingNodeGenericResources(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"node-generic-resources": ["foo=bar", "bar=baz"]}`))
	defer tempFile.Remove()
	configFile := tempFile.Path()

	opts := defaultOptions(t, configFile)
	flags := opts.flags

	assert.Check(t, flags.Set("config-file", configFile))
	assert.Check(t, flags.Set("node-generic-resource", "r1=bar"))
	assert.Check(t, flags.Set("node-generic-resource", "r2=baz"))

	_, err := loadDaemonCliConfig(opts)
	assert.Check(t, is.ErrorContains(err, "as a flag and in the configuration file: node-generic-resources"))
}

func TestLoadDaemonCliWithConflictingLabels(t *testing.T) {
	opts := defaultOptions(t, "")
	flags := opts.flags

	assert.Check(t, flags.Set("label", "foo=bar"))
	assert.Check(t, flags.Set("label", "foo=baz"))

	_, err := loadDaemonCliConfig(opts)
	assert.Check(t, is.Error(err, "conflict labels for foo=baz and foo=bar"))
}

func TestLoadDaemonCliWithDuplicateLabels(t *testing.T) {
	opts := defaultOptions(t, "")
	flags := opts.flags

	assert.Check(t, flags.Set("label", "foo=the-same"))
	assert.Check(t, flags.Set("label", "foo=the-same"))

	_, err := loadDaemonCliConfig(opts)
	assert.Check(t, err)
}

func TestLoadDaemonCliConfigWithTLSVerify(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"tlsverify": true}`))
	defer tempFile.Remove()

	opts := defaultOptions(t, tempFile.Path())
	opts.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	assert.Check(t, is.Equal(*loadedConfig.TLS, true))
}

func TestLoadDaemonCliConfigWithExplicitTLSVerifyFalse(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"tlsverify": false}`))
	defer tempFile.Remove()

	opts := defaultOptions(t, tempFile.Path())
	opts.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	assert.Check(t, *loadedConfig.TLS)
}

func TestLoadDaemonCliConfigWithoutTLSVerify(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{}`))
	defer tempFile.Remove()

	opts := defaultOptions(t, tempFile.Path())
	opts.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	assert.Check(t, loadedConfig.TLS == nil)
}

func TestLoadDaemonCliConfigWithLogLevel(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"log-level": "warn"}`))
	defer tempFile.Remove()

	opts := defaultOptions(t, tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	assert.Check(t, is.Equal("warn", loadedConfig.LogLevel))
}

func TestLoadDaemonConfigWithEmbeddedOptions(t *testing.T) {
	content := `{"tlscacert": "/etc/certs/ca.pem", "log-driver": "syslog"}`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))
	defer tempFile.Remove()

	opts := defaultOptions(t, tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	assert.Check(t, is.Equal("/etc/certs/ca.pem", loadedConfig.CommonTLSOptions.CAFile))
	assert.Check(t, is.Equal("syslog", loadedConfig.LogConfig.Type))
}

func TestLoadDaemonConfigWithRegistryOptions(t *testing.T) {
	content := `{
		"allow-nondistributable-artifacts": ["allow-nondistributable-artifacts.example.com"],
		"registry-mirrors": ["https://mirrors.example.com"],
		"insecure-registries": ["https://insecure-registry.example.com"]
	}`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))
	defer tempFile.Remove()

	opts := defaultOptions(t, tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)

	assert.Check(t, is.Len(loadedConfig.AllowNondistributableArtifacts, 1))
	assert.Check(t, is.Len(loadedConfig.Mirrors, 1))
	assert.Check(t, is.Len(loadedConfig.InsecureRegistries, 1))
}

func TestConfigureDaemonLogs(t *testing.T) {
	conf := &config.Config{}
	configureDaemonLogs(conf)
	assert.Check(t, is.Equal(logrus.InfoLevel, logrus.GetLevel()))

	conf.LogLevel = "warn"
	configureDaemonLogs(conf)
	assert.Check(t, is.Equal(logrus.WarnLevel, logrus.GetLevel()))

	// log level should not be changed when passing an invalid value
	conf.LogLevel = "foobar"
	configureDaemonLogs(conf)
	assert.Check(t, is.Equal(logrus.WarnLevel, logrus.GetLevel()))
}
