package main

import (
	"runtime"
	"testing"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon/config"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
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
	installConfigFlags(opts.daemonConfig, opts.flags)
	opts.flags.StringVar(&opts.configFile, "config-file", opts.configFile, "")
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
	assert.Check(t, is.Equal("/tmp/ca.pem", loadedConfig.TLSOptions.CAFile))
}

func TestLoadDaemonCliConfigWithConflicts(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"labels": ["l3=foo"]}`))
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

	opts := defaultOptions(t, tempFile.Path())
	opts.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	assert.Check(t, is.Equal(*loadedConfig.TLS, true))
}

func TestLoadDaemonCliConfigWithExplicitTLSVerifyFalse(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"tlsverify": false}`))

	opts := defaultOptions(t, tempFile.Path())
	opts.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	assert.Check(t, *loadedConfig.TLS)
}

func TestLoadDaemonCliConfigWithoutTLSVerify(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{}`))

	opts := defaultOptions(t, tempFile.Path())
	opts.TLSOptions.CAFile = "/tmp/ca.pem"

	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	assert.Check(t, is.Nil(loadedConfig.TLS))
}

func TestLoadDaemonCliConfigWithLogLevel(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"log-level": "warn"}`))

	opts := defaultOptions(t, tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	assert.Check(t, is.Equal("warn", loadedConfig.LogLevel))
}

func TestLoadDaemonCliConfigWithLogFormat(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"log-format": "json"}`))
	defer tempFile.Remove()

	opts := defaultOptions(t, tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	assert.Check(t, is.Equal(log.JSONFormat, loadedConfig.LogFormat))
}

func TestLoadDaemonCliConfigWithInvalidLogFormat(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"log-format": "foo"}`))
	defer tempFile.Remove()

	opts := defaultOptions(t, tempFile.Path())
	_, err := loadDaemonCliConfig(opts)
	assert.Check(t, is.ErrorContains(err, "invalid log format: foo"))
}

func TestLoadDaemonConfigWithEmbeddedOptions(t *testing.T) {
	content := `{"tlscacert": "/etc/certs/ca.pem", "log-driver": "syslog"}`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))

	opts := defaultOptions(t, tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)
	assert.Check(t, is.Equal("/etc/certs/ca.pem", loadedConfig.TLSOptions.CAFile))
	assert.Check(t, is.Equal("syslog", loadedConfig.LogConfig.Type))
}

func TestLoadDaemonConfigWithRegistryOptions(t *testing.T) {
	content := `{
		"registry-mirrors": ["https://mirrors.example.com"],
		"insecure-registries": ["https://insecure-registry.example.com"]
	}`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))

	opts := defaultOptions(t, tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	assert.NilError(t, err)
	assert.Assert(t, loadedConfig != nil)

	assert.Check(t, is.Len(loadedConfig.Mirrors, 1))
	assert.Check(t, is.Len(loadedConfig.InsecureRegistries, 1))
}

func TestConfigureDaemonLogs(t *testing.T) {
	conf := &config.Config{}
	configureDaemonLogs(conf)
	assert.Check(t, is.Equal(log.InfoLevel, log.GetLevel()))

	// log level should not be changed when passing an invalid value
	conf.LogLevel = "foobar"
	configureDaemonLogs(conf)
	assert.Check(t, is.Equal(log.InfoLevel, log.GetLevel()))

	conf.LogLevel = "warn"
	configureDaemonLogs(conf)
	assert.Check(t, is.Equal(log.WarnLevel, log.GetLevel()))
}

func TestCDISpecDirs(t *testing.T) {
	testCases := []struct {
		description         string
		configContent       string
		specDirs            []string
		expectedCDISpecDirs []string
	}{
		{
			description:         "CDI enabled by default",
			specDirs:            nil,
			expectedCDISpecDirs: []string{"/etc/cdi", "/var/run/cdi"},
		},
		{
			description:         "CDI explicitly enabled and no spec dirs specified returns default",
			specDirs:            nil,
			configContent:       `{"features": {"cdi": true}}`,
			expectedCDISpecDirs: []string{"/etc/cdi", "/var/run/cdi"},
		},
		{
			description:         "CDI enabled and specified spec dirs are returned",
			specDirs:            []string{"/foo/bar", "/baz/qux"},
			configContent:       `{"features": {"cdi": true}}`,
			expectedCDISpecDirs: []string{"/foo/bar", "/baz/qux"},
		},
		{
			description:         "CDI enabled and empty string as spec dir returns empty slice",
			specDirs:            []string{""},
			configContent:       `{"features": {"cdi": true}}`,
			expectedCDISpecDirs: []string{},
		},
		{
			description:         "CDI enabled and empty config option returns empty slice",
			configContent:       `{"cdi-spec-dirs": [], "features": {"cdi": true}}`,
			expectedCDISpecDirs: []string{},
		},
		{
			description:         "CDI disabled and no spec dirs specified returns no cdi spec dirs",
			specDirs:            nil,
			configContent:       `{"features": {"cdi": false}}`,
			expectedCDISpecDirs: nil,
		},
		{
			description:         "CDI disabled and specified spec dirs returns no cdi spec dirs",
			specDirs:            []string{"/foo/bar", "/baz/qux"},
			configContent:       `{"features": {"cdi": false}}`,
			expectedCDISpecDirs: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tempFile := fs.NewFile(t, "config", fs.WithContent(tc.configContent))
			defer tempFile.Remove()

			opts := defaultOptions(t, tempFile.Path())

			flags := opts.flags
			for _, specDir := range tc.specDirs {
				assert.Check(t, flags.Set("cdi-spec-dir", specDir))
			}

			loadedConfig, err := loadDaemonCliConfig(opts)
			assert.NilError(t, err)

			assert.Check(t, is.DeepEqual(tc.expectedCDISpecDirs, loadedConfig.CDISpecDirs, cmpopts.EquateEmpty()))
		})
	}
}

// TestOtelMeterLeak is a regression test for a memory leak in the OTEL meter
// implementation that was fixed in OTEL v1.30.0.
//
// See:
// - https://github.com/open-telemetry/opentelemetry-go-contrib/issues/5190
// - https://github.com/moby/moby/pull/48690
// - https://github.com/moby/moby/issues/48144
func TestOtelMeterLeak(t *testing.T) {
	meter := otel.Meter("foo")

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	const counters = 10 * 1000 * 1000
	for i := 0; i < counters; i++ {
		_, _ = meter.Int64Counter("bar")
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	allocs := after.Mallocs - before.Mallocs
	t.Log("Allocations:", allocs)

	// currently, with OTel v1.31.0, allocations is 3; add some margin to
	// check for unexpectedly more than that.
	if allocs > 10 {
		t.Fatalf("Possible OTel leak; got more than 10 allocations (allocs: %d).", allocs)
	}
}
