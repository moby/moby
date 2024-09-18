//go:build !windows

package runconfig // import "github.com/docker/docker/runconfig"

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/sysinfo"
)

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
