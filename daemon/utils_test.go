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

	out, err := mergeLxcConfIntoOptions(hostConfig)
	if err != nil {
		t.Fatalf("Failed to merge Lxc Config: %s", err)
	}

	cpuset := out[0]
	if expected := "cgroups.cpuset=1,2"; cpuset != expected {
		t.Fatalf("expected %s got %s", expected, cpuset)
	}
}
