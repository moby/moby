//go:build !windows

package runconfig // import "github.com/docker/docker/runconfig"

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/sysinfo"
	"gotest.tools/v3/assert"
)

func TestDecodeHostConfig(t *testing.T) {
	fixtures := []struct {
		file string
	}{
		{"fixtures/unix/container_hostconfig_1_14.json"},
		{"fixtures/unix/container_hostconfig_1_19.json"},
	}

	for _, f := range fixtures {
		b, err := os.ReadFile(f.file)
		if err != nil {
			t.Fatal(err)
		}

		c, err := decodeHostConfig(bytes.NewReader(b))
		if err != nil {
			t.Fatal(fmt.Errorf("Error parsing %s: %v", f, err))
		}

		assert.Check(t, !c.Privileged)

		if l := len(c.Binds); l != 1 {
			t.Fatalf("Expected 1 bind, found %d\n", l)
		}

		if len(c.CapAdd) != 1 && c.CapAdd[0] != "NET_ADMIN" {
			t.Fatalf("Expected CapAdd NET_ADMIN, got %v", c.CapAdd)
		}

		if len(c.CapDrop) != 1 && c.CapDrop[0] != "NET_ADMIN" {
			t.Fatalf("Expected CapDrop NET_ADMIN, got %v", c.CapDrop)
		}
	}
}

func TestValidateResources(t *testing.T) {
	type resourceTest struct {
		ConfigCPURealtimePeriod  int64
		ConfigCPURealtimeRuntime int64
		SysInfoCPURealtime       bool
		ErrorExpected            bool
		FailureMsg               string
	}

	tests := []resourceTest{
		{
			ConfigCPURealtimePeriod:  1000,
			ConfigCPURealtimeRuntime: 1000,
			SysInfoCPURealtime:       true,
			ErrorExpected:            false,
			FailureMsg:               "Expected valid configuration",
		},
		{
			ConfigCPURealtimePeriod:  5000,
			ConfigCPURealtimeRuntime: 5000,
			SysInfoCPURealtime:       false,
			ErrorExpected:            true,
			FailureMsg:               "Expected failure when cpu-rt-period is set but kernel doesn't support it",
		},
		{
			ConfigCPURealtimePeriod:  5000,
			ConfigCPURealtimeRuntime: 5000,
			SysInfoCPURealtime:       false,
			ErrorExpected:            true,
			FailureMsg:               "Expected failure when cpu-rt-runtime is set but kernel doesn't support it",
		},
		{
			ConfigCPURealtimePeriod:  5000,
			ConfigCPURealtimeRuntime: 10000,
			SysInfoCPURealtime:       true,
			ErrorExpected:            true,
			FailureMsg:               "Expected failure when cpu-rt-runtime is greater than cpu-rt-period",
		},
	}

	for _, rt := range tests {
		var hc container.HostConfig
		hc.Resources.CPURealtimePeriod = rt.ConfigCPURealtimePeriod
		hc.Resources.CPURealtimeRuntime = rt.ConfigCPURealtimeRuntime

		var si sysinfo.SysInfo
		si.CPURealtime = rt.SysInfoCPURealtime

		if err := validateResources(&hc, &si); (err != nil) != rt.ErrorExpected {
			t.Fatal(rt.FailureMsg, err)
		}
	}
}
