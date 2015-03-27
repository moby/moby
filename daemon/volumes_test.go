package daemon

import (
	"testing"

	"github.com/docker/docker/runconfig"
)

func TestValidateVolumePath(t *testing.T) {

	config := &runconfig.Config{Volumes: make(map[string]struct{})}
	hostConfig := &runconfig.HostConfig{}

	// given invalid src
	hostConfig.Binds = []string{"/data:data"}
	if err := validateVolumePath(config, hostConfig); err == nil {
		t.Fatal("Expected validateVolumePath error, got nil")
	}

	// given invalid dest
	hostConfig.Binds = []string{"data:/data"}
	if err := validateVolumePath(config, hostConfig); err == nil {
		t.Fatal("Expected validateVolumePath error, got nil")
	}

	// given invalid mode
	hostConfig.Binds = []string{"/data:/data:rr"}
	if err := validateVolumePath(config, hostConfig); err == nil {
		t.Fatal("Expected validateVolumePath error, got nil")
	}

	// given invalid format
	hostConfig.Binds = []string{"/data:/data:/data:ro"}
	if err := validateVolumePath(config, hostConfig); err == nil {
		t.Fatal("Expected validateVolumePath error, got nil")
	}

	hostConfig.Binds = []string{"/data"}
	if err := validateVolumePath(config, hostConfig); err == nil {
		t.Fatal("Expected validateVolumePath error, got nil")
	}

	// given invalid path
	hostConfig.Binds = []string{}
	config.Volumes["data"] = struct{}{}
	if err := validateVolumePath(config, hostConfig); err == nil {
		t.Fatal("Expected validateVolumePath error, got nil")
	}

	// given invalid volume-from id
	hostConfig.Binds = []string{}
	config.Volumes = make(map[string]struct{})
	hostConfig.VolumesFrom = []string{""}
	if err := validateVolumePath(config, hostConfig); err == nil {
		t.Fatal("Expected validateVolumePath error, got nil")
	}

	// given invalid volume-from mode
	hostConfig.Binds = []string{}
	config.Volumes = make(map[string]struct{})
	hostConfig.VolumesFrom = []string{"con:rr"}
	if err := validateVolumePath(config, hostConfig); err == nil {
		t.Fatal("Expected validateVolumePath error, got nil")
	}

	// given success path
	hostConfig.Binds = []string{"/data1:/data2", "/data3:/data4:rw", "/data5:/data6:ro"}
	config.Volumes = make(map[string]struct{})
	config.Volumes["/data7"] = struct{}{}
	hostConfig.VolumesFrom = []string{"con1", "con2:rw", "con3:ro"}
	if err := validateVolumePath(config, hostConfig); err != nil {
		t.Fatal("Expected no validateVolumePath error, got one: ", err)
	}

	t.Log("parseVolumeMountConfig test passed")
}
