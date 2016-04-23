package main

import (
	"io/ioutil"
	"os"
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
	defer os.Remove(configFile)

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
	defer os.Remove(configFile)

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
	defer os.Remove(configFile)

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
	defer os.Remove(configFile)

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
	defer os.Remove(configFile)

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
	defer os.Remove(configFile)

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

func TestLoadDaemonConfigWithRegistryOptions(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{}
	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	c.ServiceOptions.InstallCliFlags(flags, absentFromHelp)

	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}
	configFile := f.Name()
	defer os.Remove(configFile)

	f.Write([]byte(`{"registry-mirrors": ["https://mirrors.docker.com"], "insecure-registries": ["https://insecure.docker.com"], "disable-legacy-registry": true}`))
	f.Close()

	loadedConfig, err := loadDaemonCliConfig(c, flags, common, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig == nil {
		t.Fatal("expected configuration, got nil")
	}

	m := loadedConfig.Mirrors
	if len(m) != 1 {
		t.Fatalf("expected 1 mirror, got %d", len(m))
	}

	r := loadedConfig.InsecureRegistries
	if len(r) != 1 {
		t.Fatalf("expected 1 insecure registries, got %d", len(r))
	}

	if !loadedConfig.V2Only {
		t.Fatal("expected disable-legacy-registry to be true, got false")
	}
}
