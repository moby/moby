package daemon

import (
	"fmt"
	"testing"

	"github.com/Microsoft/hcsshim/osversion"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCheckImageCompatibilityForHostIsolation(t *testing.T) {
	testCases := []struct {
		hostBuildNo    uint16
		imageBuildNo   uint16
		isHyperV       bool
		expectedErrMsg string
	}{
		// coincinding build numbers are valid regardless of isolation:
		{
			hostBuildNo:    osversion.RS1,
			imageBuildNo:   osversion.RS1,
			isHyperV:       false,
			expectedErrMsg: "",
		},
		{
			hostBuildNo:    osversion.RS1,
			imageBuildNo:   osversion.RS1,
			isHyperV:       true,
			expectedErrMsg: "",
		},
		// images older than the host must be run using Hyper-V:
		{
			hostBuildNo:    osversion.RS2,
			imageBuildNo:   osversion.RS1,
			isHyperV:       false,
			expectedErrMsg: "an older Windows version .*-based image can only be run on a .* host using Hyper-V isolation",
		},
		{
			hostBuildNo:    osversion.RS2,
			imageBuildNo:   osversion.RS1,
			isHyperV:       true,
			expectedErrMsg: "",
		},
		// images newer than the host can not run prior to host version RS5:
		{
			hostBuildNo:    osversion.RS4,
			imageBuildNo:   osversion.RS5,
			isHyperV:       false,
			expectedErrMsg: "a Windows version .*-based image is incompatible with a .* host",
		},
		{
			hostBuildNo:    osversion.RS4,
			imageBuildNo:   osversion.RS5,
			isHyperV:       true,
			expectedErrMsg: "a Windows version .*-based image is incompatible with a .* host",
		},
		// images newer than an RS5+ host can only run using Hyper-V:
		{
			hostBuildNo:    osversion.RS5,
			imageBuildNo:   osversion.V21H2Server, // ltsc2022
			isHyperV:       false,
			expectedErrMsg: "a newer Windows version .*-based image can only be run on a .* host using Hyper-V isolation",
		},
		{
			hostBuildNo:    osversion.RS5,
			imageBuildNo:   osversion.V21H2Server,
			isHyperV:       true,
			expectedErrMsg: "",
		},
	}

	for _, testCase := range testCases {
		hostVersion := osversion.OSVersion{
			Version:      0,
			MajorVersion: 0,
			MinorVersion: 0,
			Build:        testCase.hostBuildNo,
		}
		imageOSVersion := fmt.Sprintf("10.0.%d", testCase.imageBuildNo)
		err := checkImageCompatibilityForHostIsolation(hostVersion, imageOSVersion, testCase.isHyperV)
		if testCase.expectedErrMsg == "" {
			assert.NilError(t, err)
		} else {
			assert.Check(t, is.Regexp(testCase.expectedErrMsg, err.Error()))
		}
	}
}
