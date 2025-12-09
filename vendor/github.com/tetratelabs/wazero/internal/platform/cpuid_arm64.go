//go:build gc

package platform

import "runtime"

// CpuFeatures exposes the capabilities for this CPU, queried via the Has, HasExtra methods.
var CpuFeatures = loadCpuFeatureFlags()

// cpuFeatureFlags implements CpuFeatureFlags interface.
type cpuFeatureFlags struct {
	isar0 uint64
	isar1 uint64
}

// implemented in cpuid_arm64.s
func getisar0() uint64

// implemented in cpuid_arm64.s
func getisar1() uint64

func loadCpuFeatureFlags() CpuFeatureFlags {
	switch runtime.GOOS {
	case "darwin", "windows":
		// These OSes do not allow userland to read the instruction set attribute registers,
		// but basically require atomic instructions:
		// - "darwin" is the desktop version (mobile version is "ios"),
		//   and the M1 is a ARMv8.4.
		// - "windows" requires them from Windows 11, see page 12
		//   https://download.microsoft.com/download/7/8/8/788bf5ab-0751-4928-a22c-dffdc23c27f2/Minimum%20Hardware%20Requirements%20for%20Windows%2011.pdf
		return &cpuFeatureFlags{
			isar0: uint64(CpuFeatureArm64Atomic),
			isar1: 0,
		}
	case "linux", "freebsd":
		// These OSes allow userland to read the instruction set attribute registers,
		// which is otherwise restricted to EL0:
		// https://kernel.org/doc/Documentation/arm64/cpu-feature-registers.txt
		// See these for contents of the registers:
		// https://developer.arm.com/documentation/ddi0601/latest/AArch64-Registers/ID-AA64ISAR0-EL1--AArch64-Instruction-Set-Attribute-Register-0
		// https://developer.arm.com/documentation/ddi0601/latest/AArch64-Registers/ID-AA64ISAR1-EL1--AArch64-Instruction-Set-Attribute-Register-1
		return &cpuFeatureFlags{
			isar0: getisar0(),
			isar1: getisar1(),
		}
	default:
		return &cpuFeatureFlags{}
	}
}

// Has implements the same method on the CpuFeatureFlags interface.
func (f *cpuFeatureFlags) Has(cpuFeature CpuFeature) bool {
	return (f.isar0 & uint64(cpuFeature)) != 0
}

// HasExtra implements the same method on the CpuFeatureFlags interface.
func (f *cpuFeatureFlags) HasExtra(cpuFeature CpuFeature) bool {
	return (f.isar1 & uint64(cpuFeature)) != 0
}

// Raw implements the same method on the CpuFeatureFlags interface.
func (f *cpuFeatureFlags) Raw() uint64 {
	// Below, we only set bits for the features we care about,
	// instead of setting all the unnecessary bits obtained from the
	// instruction set attribute registers.
	var ret uint64
	if f.Has(CpuFeatureArm64Atomic) {
		ret = 1 << 0
	}
	return ret
}
