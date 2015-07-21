// +build linux

package server

import (
	"testing"

	"github.com/docker/docker/pkg/version"
	"github.com/docker/docker/runconfig"
)

func TestAdjustCPUSharesOldApi(t *testing.T) {
	apiVersion := version.Version("1.18")
	hostConfig := &runconfig.HostConfig{
		CPUShares: linuxMinCPUShares - 1,
	}
	adjustCPUShares(apiVersion, hostConfig)
	if hostConfig.CPUShares != linuxMinCPUShares {
		t.Errorf("Expected CPUShares to be %d", linuxMinCPUShares)
	}

	hostConfig.CPUShares = linuxMaxCPUShares + 1
	adjustCPUShares(apiVersion, hostConfig)
	if hostConfig.CPUShares != linuxMaxCPUShares {
		t.Errorf("Expected CPUShares to be %d", linuxMaxCPUShares)
	}

	hostConfig.CPUShares = 0
	adjustCPUShares(apiVersion, hostConfig)
	if hostConfig.CPUShares != 0 {
		t.Error("Expected CPUShares to be unchanged")
	}

	hostConfig.CPUShares = 1024
	adjustCPUShares(apiVersion, hostConfig)
	if hostConfig.CPUShares != 1024 {
		t.Error("Expected CPUShares to be unchanged")
	}
}

func TestAdjustCPUSharesNoAdjustment(t *testing.T) {
	apiVersion := version.Version("1.19")
	hostConfig := &runconfig.HostConfig{
		CPUShares: linuxMinCPUShares - 1,
	}
	adjustCPUShares(apiVersion, hostConfig)
	if hostConfig.CPUShares != linuxMinCPUShares-1 {
		t.Errorf("Expected CPUShares to be %d", linuxMinCPUShares-1)
	}

	hostConfig.CPUShares = linuxMaxCPUShares + 1
	adjustCPUShares(apiVersion, hostConfig)
	if hostConfig.CPUShares != linuxMaxCPUShares+1 {
		t.Errorf("Expected CPUShares to be %d", linuxMaxCPUShares+1)
	}

	hostConfig.CPUShares = 0
	adjustCPUShares(apiVersion, hostConfig)
	if hostConfig.CPUShares != 0 {
		t.Error("Expected CPUShares to be unchanged")
	}

	hostConfig.CPUShares = 1024
	adjustCPUShares(apiVersion, hostConfig)
	if hostConfig.CPUShares != 1024 {
		t.Error("Expected CPUShares to be unchanged")
	}
}
