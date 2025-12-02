//go:build gc

package platform

// CpuFeatures exposes the capabilities for this CPU, queried via the Has, HasExtra methods.
var CpuFeatures = loadCpuFeatureFlags()

// cpuFeatureFlags implements CpuFeatureFlags interface.
type cpuFeatureFlags struct {
	flags      uint64
	extraFlags uint64
}

// cpuid exposes the CPUID instruction to the Go layer (https://www.amd.com/system/files/TechDocs/25481.pdf)
// implemented in cpuid_amd64.s
func cpuid(arg1, arg2 uint32) (eax, ebx, ecx, edx uint32)

// cpuidAsBitmap combines the result of invoking cpuid to uint64 bitmap.
func cpuidAsBitmap(arg1, arg2 uint32) uint64 {
	_ /* eax */, _ /* ebx */, ecx, edx := cpuid(arg1, arg2)
	return (uint64(edx) << 32) | uint64(ecx)
}

// loadStandardRange load flags from the standard range, panics otherwise.
func loadStandardRange(id uint32) uint64 {
	// ensure that the id is in the valid range, returned by cpuid(0,0)
	maxRange, _, _, _ := cpuid(0, 0)
	if id > maxRange {
		panic("cannot query standard CPU flags")
	}
	return cpuidAsBitmap(id, 0)
}

// loadStandardRange load flags from the extended range, panics otherwise.
func loadExtendedRange(id uint32) uint64 {
	// ensure that the id is in the valid range, returned by cpuid(0x80000000,0)
	maxRange, _, _, _ := cpuid(0x80000000, 0)
	if id > maxRange {
		panic("cannot query extended CPU flags")
	}
	return cpuidAsBitmap(id, 0)
}

func loadCpuFeatureFlags() CpuFeatureFlags {
	return &cpuFeatureFlags{
		flags:      loadStandardRange(1),
		extraFlags: loadExtendedRange(0x80000001),
	}
}

// Has implements the same method on the CpuFeatureFlags interface.
func (f *cpuFeatureFlags) Has(cpuFeature CpuFeature) bool {
	return (f.flags & uint64(cpuFeature)) != 0
}

// HasExtra implements the same method on the CpuFeatureFlags interface.
func (f *cpuFeatureFlags) HasExtra(cpuFeature CpuFeature) bool {
	return (f.extraFlags & uint64(cpuFeature)) != 0
}

// Raw implements the same method on the CpuFeatureFlags interface.
func (f *cpuFeatureFlags) Raw() uint64 {
	// Below, we only set bits for the features we care about,
	// instead of setting all the unnecessary bits obtained from the
	// CPUID instruction.
	var ret uint64
	if f.Has(CpuFeatureAmd64SSE3) {
		ret = 1 << 0
	}
	if f.Has(CpuFeatureAmd64SSE4_1) {
		ret |= 1 << 1
	}
	if f.Has(CpuFeatureAmd64SSE4_2) {
		ret |= 1 << 2
	}
	if f.HasExtra(CpuExtraFeatureAmd64ABM) {
		ret |= 1 << 3
	}
	return ret
}
