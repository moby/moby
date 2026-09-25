package platform

import (
	"runtime"

	"golang.org/x/sys/cpu"
)

// CpuFeatures exposes the capabilities for this CPU, queried via the Has method.
var CpuFeatures = loadCpuFeatureFlags()

func loadCpuFeatureFlags() (flags CpuFeatureFlags) {
	switch runtime.GOOS {
	case "darwin", "windows":
		// These OSes do not allow userland to read the instruction set attribute registers,
		// but basically require atomic instructions:
		// - "darwin" is the desktop version (mobile version is "ios"),
		//   and the M1 is a ARMv8.4.
		// - "windows" requires them from Windows 11, see page 12
		//   https://download.microsoft.com/download/7/8/8/788bf5ab-0751-4928-a22c-dffdc23c27f2/Minimum%20Hardware%20Requirements%20for%20Windows%2011.pdf
		flags |= CpuFeatureArm64Atomic
	default:
		if cpu.ARM64.HasATOMICS {
			flags |= CpuFeatureArm64Atomic
		}
	}
	return
}
