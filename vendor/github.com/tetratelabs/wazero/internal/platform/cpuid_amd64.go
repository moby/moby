package platform

import "golang.org/x/sys/cpu"

// CpuFeatures exposes the capabilities for this CPU, queried via the Has method.
var CpuFeatures = loadCpuFeatureFlags()

func loadCpuFeatureFlags() (flags CpuFeatureFlags) {
	if cpu.X86.HasSSE41 {
		flags |= CpuFeatureAmd64SSE4_1
	}
	if cpu.X86.HasBMI1 {
		flags |= CpuFeatureAmd64BMI1
	}
	// x/sys/cpu does not track the ABM explicitly.
	// LZCNT combined with BMI1 and BMI2 completes the expanded ABM instruction set.
	// Intel includes LZCNT in BMI1, and all AMD CPUs with POPCNT also have LZCNT.
	if cpu.X86.HasBMI1 && cpu.X86.HasBMI2 && cpu.X86.HasPOPCNT {
		flags |= CpuFeatureAmd64ABM
	}
	return
}
