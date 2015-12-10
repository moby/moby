package daemon

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/mflag"
)

func TestDaemonConfigurationMerge(t *testing.T) {
	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"debug": true}`))
	f.Close()

	c := &Config{
		CommonConfig: CommonConfig{
			AutoRestart: true,
			LogConfig: LogConfig{
				Type:   "syslog",
				Config: map[string]string{"tag": "test"},
			},
		},
	}

	cc, err := MergeDaemonConfigurations(c, nil, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if !cc.Debug {
		t.Fatalf("expected %v, got %v\n", true, cc.Debug)
	}
	if !cc.AutoRestart {
		t.Fatalf("expected %v, got %v\n", true, cc.AutoRestart)
	}
	if cc.LogConfig.Type != "syslog" {
		t.Fatalf("expected syslog config, got %q\n", cc.LogConfig)
	}
}

func TestDaemonConfigurationNotFound(t *testing.T) {
	_, err := MergeDaemonConfigurations(&Config{}, nil, "/tmp/foo-bar-baz-docker")
	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected does not exist error, got %v", err)
	}
}

func TestDaemonBrokenConfiguration(t *testing.T) {
	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"Debug": tru`))
	f.Close()

	_, err = MergeDaemonConfigurations(&Config{}, nil, configFile)
	if err == nil {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestParseClusterAdvertiseSettings(t *testing.T) {
	_, err := parseClusterAdvertiseSettings("something", "")
	if err != errDiscoveryDisabled {
		t.Fatalf("expected discovery disabled error, got %v\n", err)
	}

	_, err = parseClusterAdvertiseSettings("", "something")
	if err == nil {
		t.Fatalf("expected discovery store error, got %v\n", err)
	}

	_, err = parseClusterAdvertiseSettings("etcd", "127.0.0.1:8080")
	if err != nil {
		t.Fatal(err)
	}
}

func TestFindConfigurationConflicts(t *testing.T) {
	config := map[string]interface{}{"authorization-plugins": "foobar"}
	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)

	err := findConfigurationConflicts(config, flags)
	if err != nil {
		t.Fatal(err)
	}

	flags.String([]string{"authorization-plugins"}, "", "")
	if err := flags.Set("authorization-plugins", "asdf"); err != nil {
		t.Fatal(err)
	}

	err = findConfigurationConflicts(config, flags)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "authorization-plugins") {
		t.Fatalf("expected authorization-plugins conflict, got %v", err)
	}
}

func TestFindConfigurationConflictsWithNamedOptions(t *testing.T) {
	config := map[string]interface{}{"hosts": []string{"qwer"}}
	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)

	var hosts []string
	flags.Var(opts.NewNamedListOptsRef("hosts", &hosts, opts.ValidateHost), []string{"H", "-host"}, "Daemon socket(s) to connect to")
	if err := flags.Set("-host", "tcp://127.0.0.1:4444"); err != nil {
		t.Fatal(err)
	}
	if err := flags.Set("H", "unix:///var/run/docker.sock"); err != nil {
		t.Fatal(err)
	}

	err := findConfigurationConflicts(config, flags)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "hosts") {
		t.Fatalf("expected hosts conflict, got %v", err)
	}
}

func TestDaemonConfigurationMergeConflicts(t *testing.T) {
	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"debug": true}`))
	f.Close()

	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	flags.Bool([]string{"debug"}, false, "")
	flags.Set("debug", "false")

	_, err = MergeDaemonConfigurations(&Config{}, flags, configFile)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "debug") {
		t.Fatalf("expected debug conflict, got %v", err)
	}
}

func TestDaemonConfigurationMergeConflictsWithInnerStructs(t *testing.T) {
	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"tlscacert": "/etc/certificates/ca.pem"}`))
	f.Close()

	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	flags.String([]string{"tlscacert"}, "", "")
	flags.Set("tlscacert", "~/.docker/ca.pem")

	_, err = MergeDaemonConfigurations(&Config{}, flags, configFile)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tlscacert") {
		t.Fatalf("expected tlscacert conflict, got %v", err)
	}
}
