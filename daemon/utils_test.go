package daemon

import (
	"testing"

	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

func TestMergeLxcConfig(t *testing.T) {
	var (
		hostConfig = &runconfig.HostConfig{
			LxcConf: []utils.KeyValuePair{
				{Key: "lxc.cgroups.cpuset", Value: "1,2"},
			},
		}
		driverConfig = make(map[string][]string)
	)

	mergeLxcConfIntoOptions(hostConfig, driverConfig)
	if l := len(driverConfig["lxc"]); l > 1 {
		t.Fatalf("expected lxc options len of 1 got %d", l)
	}

	cpuset := driverConfig["lxc"][0]
	if expected := "cgroups.cpuset=1,2"; cpuset != expected {
		t.Fatalf("expected %s got %s", expected, cpuset)
	}
}
