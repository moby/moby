package daemon

import (
	"testing"

	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

func TestMergeLxcConfig(t *testing.T) {
	hostConfig := &runconfig.HostConfig{
		LxcConf: []utils.KeyValuePair{
			{Key: "lxc.cgroups.cpuset", Value: "1,2"},
		},
	}

	out := mergeLxcConfIntoOptions(hostConfig)

	cpuset := out[0]
	if expected := "cgroups.cpuset=1,2"; cpuset != expected {
		t.Fatalf("expected %s got %s", expected, cpuset)
	}
}

func TestRemoveLocalDns(t *testing.T) {
	ns0 := "nameserver 10.16.60.14\nnameserver 10.16.60.21\n"

	if result := utils.RemoveLocalDns([]byte(ns0)); result != nil {
		if ns0 != string(result) {
			t.Fatalf("Failed No Localhost: expected \n<%s> got \n<%s>", ns0, string(result))
		}
	}

	ns1 := "nameserver 10.16.60.14\nnameserver 10.16.60.21\nnameserver 127.0.0.1\n"
	if result := utils.RemoveLocalDns([]byte(ns1)); result != nil {
		if ns0 != string(result) {
			t.Fatalf("Failed Localhost: expected \n<%s> got \n<%s>", ns0, string(result))
		}
	}

	ns1 = "nameserver 10.16.60.14\nnameserver 127.0.0.1\nnameserver 10.16.60.21\n"
	if result := utils.RemoveLocalDns([]byte(ns1)); result != nil {
		if ns0 != string(result) {
			t.Fatalf("Failed Localhost: expected \n<%s> got \n<%s>", ns0, string(result))
		}
	}

	ns1 = "nameserver 127.0.1.1\nnameserver 10.16.60.14\nnameserver 10.16.60.21\n"
	if result := utils.RemoveLocalDns([]byte(ns1)); result != nil {
		if ns0 != string(result) {
			t.Fatalf("Failed Localhost: expected \n<%s> got \n<%s>", ns0, string(result))
		}
	}
}
