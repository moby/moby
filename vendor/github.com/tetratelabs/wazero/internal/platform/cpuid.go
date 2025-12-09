package platform

// CpuFeatureFlags exposes methods for querying CPU capabilities
type CpuFeatureFlags interface {
	// Has returns true when the specified flag (represented as uint64) is supported
	Has(cpuFeature CpuFeature) bool
	// HasExtra returns true when the specified extraFlag (represented as uint64) is supported
	HasExtra(cpuFeature CpuFeature) bool
	// Raw returns the raw bitset that represents CPU features used by wazero. This can be used for cache keying.
	// For now, we only use four features, so uint64 is enough.
	Raw() uint64
}

type CpuFeature uint64

const (
	// CpuFeatureAmd64SSE3 is the flag to query CpuFeatureFlags.Has for SSEv3 capabilities on amd64
	CpuFeatureAmd64SSE3 CpuFeature = 1
	// CpuFeatureAmd64SSE4_1 is the flag to query CpuFeatureFlags.Has for SSEv4.1 capabilities on amd64
	CpuFeatureAmd64SSE4_1 CpuFeature = 1 << 19
	// CpuFeatureAmd64SSE4_2 is the flag to query CpuFeatureFlags.Has for SSEv4.2 capabilities on amd64
	CpuFeatureAmd64SSE4_2 CpuFeature = 1 << 20
	// Note: when adding new features, ensure that the feature is included in CpuFeatureFlags.Raw.
)

const (
	// CpuExtraFeatureAmd64ABM is the flag to query CpuFeatureFlags.HasExtra for Advanced Bit Manipulation capabilities (e.g. LZCNT) on amd64
	CpuExtraFeatureAmd64ABM CpuFeature = 1 << 5
	// Note: when adding new features, ensure that the feature is included in CpuFeatureFlags.Raw.
)

const (
	// CpuFeatureArm64Atomic is the flag to query CpuFeatureFlags.Has for Large System Extensions capabilities on arm64
	CpuFeatureArm64Atomic CpuFeature = 1 << 21
)
