// +build daemon,!windows

package main

import (
	"io/ioutil"
	"testing"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/mflag"
)

func TestLoadDaemonConfigWithNetwork(t *testing.T) {
	c := &daemon.Config{}
	common := &cli.CommonFlags{}
	flags := mflag.NewFlagSet("test", mflag.ContinueOnError)
	flags.String([]string{"-bip"}, "", "")
	flags.String([]string{"-ip"}, "", "")

	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"bip": "127.0.0.2", "ip": "127.0.0.1"}`))
	f.Close()

	loadedConfig, err := loadDaemonCliConfig(c, flags, common, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig == nil {
		t.Fatalf("expected configuration %v, got nil", c)
	}
	if loadedConfig.IP != "127.0.0.2" {
		t.Fatalf("expected IP 127.0.0.2, got %v", loadedConfig.IP)
	}
	if loadedConfig.DefaultIP.String() != "127.0.0.1" {
		t.Fatalf("expected DefaultIP 127.0.0.1, got %s", loadedConfig.DefaultIP)
	}
}
