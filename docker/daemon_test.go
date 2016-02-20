// +build daemon

package main

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/mflag"
	"github.com/docker/go-connections/tlsconfig"
)

func TestLoadDaemonCliConfigWithoutOverriding(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{
		Debug: true,
	}

	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	loadedConfig, err := loadDaemonCliConfig(c, flags, common, "/tmp/fooobarbaz")
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig == nil {
		t.Fatalf("expected configuration %v, got nil", c)
	}
	if !loadedConfig.Debug {
		t.Fatalf("expected debug to be copied from the common flags, got false")
	}
}

func TestLoadDaemonCliConfigWithTLS(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{
		TLS: true,
		TLSOptions: &tlsconfig.Options{
			CAFile: "/tmp/ca.pem",
		},
	}

	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	loadedConfig, err := loadDaemonCliConfig(c, flags, common, "/tmp/fooobarbaz")
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig == nil {
		t.Fatalf("expected configuration %v, got nil", c)
	}
	if loadedConfig.CommonTLSOptions.CAFile != "/tmp/ca.pem" {
		t.Fatalf("expected /tmp/ca.pem, got %s: %q", loadedConfig.CommonTLSOptions.CAFile, loadedConfig)
	}
}

func TestLoadDaemonCliConfigWithConflicts(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{}
	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"labels": ["l3=foo"]}`))
	f.Close()

	var labels []string

	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	flags.String([]string{daemonConfigFileFlag}, "", "")
	flags.Var(opts.NewNamedListOptsRef("labels", &labels, opts.ValidateLabel), []string{"-label"}, "")

	flags.Set(daemonConfigFileFlag, configFile)
	if err := flags.Set("-label", "l1=bar"); err != nil {
		t.Fatal(err)
	}
	if err := flags.Set("-label", "l2=baz"); err != nil {
		t.Fatal(err)
	}

	_, err = loadDaemonCliConfig(c, flags, common, configFile)
	if err == nil {
		t.Fatalf("expected configuration error, got nil")
	}
	if !strings.Contains(err.Error(), "labels") {
		t.Fatalf("expected labels conflict, got %v", err)
	}
}

func TestLoadDaemonCliConfigWithTLSVerify(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{
		TLSOptions: &tlsconfig.Options{
			CAFile: "/tmp/ca.pem",
		},
	}

	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"tlsverify": true}`))
	f.Close()

	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	flags.Bool([]string{"-tlsverify"}, false, "")
	loadedConfig, err := loadDaemonCliConfig(c, flags, common, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig == nil {
		t.Fatalf("expected configuration %v, got nil", c)
	}

	if !loadedConfig.TLS {
		t.Fatalf("expected TLS enabled, got %q", loadedConfig)
	}
}

func TestLoadDaemonCliConfigWithExplicitTLSVerifyFalse(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{
		TLSOptions: &tlsconfig.Options{
			CAFile: "/tmp/ca.pem",
		},
	}

	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"tlsverify": false}`))
	f.Close()

	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	flags.Bool([]string{"-tlsverify"}, false, "")
	loadedConfig, err := loadDaemonCliConfig(c, flags, common, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig == nil {
		t.Fatalf("expected configuration %v, got nil", c)
	}

	if !loadedConfig.TLS {
		t.Fatalf("expected TLS enabled, got %q", loadedConfig)
	}
}

func TestLoadDaemonCliConfigWithoutTLSVerify(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{
		TLSOptions: &tlsconfig.Options{
			CAFile: "/tmp/ca.pem",
		},
	}

	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{}`))
	f.Close()

	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	loadedConfig, err := loadDaemonCliConfig(c, flags, common, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig == nil {
		t.Fatalf("expected configuration %v, got nil", c)
	}

	if loadedConfig.TLS {
		t.Fatalf("expected TLS disabled, got %q", loadedConfig)
	}
}

func TestLoadDaemonCliConfigWithLogLevel(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{}

	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"log-level": "warn"}`))
	f.Close()

	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	flags.String([]string{"-log-level"}, "", "")
	loadedConfig, err := loadDaemonCliConfig(c, flags, common, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig == nil {
		t.Fatalf("expected configuration %v, got nil", c)
	}
	if loadedConfig.LogLevel != "warn" {
		t.Fatalf("expected warn log level, got %v", loadedConfig.LogLevel)
	}

	if logrus.GetLevel() != logrus.WarnLevel {
		t.Fatalf("expected warn log level, got %v", logrus.GetLevel())
	}
}

func TestLoadDaemonConfigWithEmbeddedOptions(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{}
	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	flags.String([]string{"-tlscacert"}, "", "")
	flags.String([]string{"-log-driver"}, "", "")

	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"tlscacert": "/etc/certs/ca.pem", "log-driver": "syslog"}`))
	f.Close()

	loadedConfig, err := loadDaemonCliConfig(c, flags, common, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig == nil {
		t.Fatal("expected configuration, got nil")
	}
	if loadedConfig.CommonTLSOptions.CAFile != "/etc/certs/ca.pem" {
		t.Fatalf("expected CA file path /etc/certs/ca.pem, got %v", loadedConfig.CommonTLSOptions.CAFile)
	}
	if loadedConfig.LogConfig.Type != "syslog" {
		t.Fatalf("expected LogConfig type syslog, got %v", loadedConfig.LogConfig.Type)
	}
}

func TestLoadDaemonConfigWithMapOptions(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{}
	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)

	flags.Var(opts.NewNamedMapOpts("cluster-store-opts", c.ClusterOpts, nil), []string{"-cluster-store-opt"}, "")
	flags.Var(opts.NewNamedMapOpts("log-opts", c.LogConfig.Config, nil), []string{"-log-opt"}, "")

	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{
		"cluster-store-opts": {"kv.cacertfile": "/var/lib/docker/discovery_certs/ca.pem"},
		"log-opts": {"tag": "test"}
}`))
	f.Close()

	loadedConfig, err := loadDaemonCliConfig(c, flags, common, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig == nil {
		t.Fatal("expected configuration, got nil")
	}
	if loadedConfig.ClusterOpts == nil {
		t.Fatal("expected cluster options, got nil")
	}

	expectedPath := "/var/lib/docker/discovery_certs/ca.pem"
	if caPath := loadedConfig.ClusterOpts["kv.cacertfile"]; caPath != expectedPath {
		t.Fatalf("expected %s, got %s", expectedPath, caPath)
	}

	if loadedConfig.LogConfig.Config == nil {
		t.Fatal("expected log config options, got nil")
	}
	if tag := loadedConfig.LogConfig.Config["tag"]; tag != "test" {
		t.Fatalf("expected log tag `test`, got %s", tag)
	}
}

func TestLoadDaemonConfigWithTrueDefaultValues(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{}
	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	flags.BoolVar(&c.EnableUserlandProxy, []string{"-userland-proxy"}, true, "")

	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	if err := flags.ParseFlags([]string{}, false); err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{
		"userland-proxy": false
}`))
	f.Close()

	loadedConfig, err := loadDaemonCliConfig(c, flags, common, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig == nil {
		t.Fatal("expected configuration, got nil")
	}

	if loadedConfig.EnableUserlandProxy {
		t.Fatal("expected userland proxy to be disabled, got enabled")
	}

	// make sure reloading doesn't generate configuration
	// conflicts after normalizing boolean values.
	err = daemon.ReloadConfiguration(configFile, flags, func(reloadedConfig *daemon.Config) {
		if reloadedConfig.EnableUserlandProxy {
			t.Fatal("expected userland proxy to be disabled, got enabled")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadDaemonConfigWithTrueDefaultValuesLeaveDefaults(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{}
	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	flags.BoolVar(&c.EnableUserlandProxy, []string{"-userland-proxy"}, true, "")

	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	if err := flags.ParseFlags([]string{}, false); err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{}`))
	f.Close()

	loadedConfig, err := loadDaemonCliConfig(c, flags, common, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig == nil {
		t.Fatal("expected configuration, got nil")
	}

	if !loadedConfig.EnableUserlandProxy {
		t.Fatal("expected userland proxy to be enabled, got disabled")
	}
}
