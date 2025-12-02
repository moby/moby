//go:build !(amd64 || arm64) || !gc

package platform

var CpuFeatures CpuFeatureFlags = &cpuFeatureFlags{}

// cpuFeatureFlags implements CpuFeatureFlags for unsupported platforms.
type cpuFeatureFlags struct{}

// Has implements the same method on the CpuFeatureFlags interface.
func (c *cpuFeatureFlags) Has(cpuFeature CpuFeature) bool { return false }

// HasExtra implements the same method on the CpuFeatureFlags interface.
func (c *cpuFeatureFlags) HasExtra(cpuFeature CpuFeature) bool { return false }

// Raw implements the same method on the CpuFeatureFlags interface.
func (c *cpuFeatureFlags) Raw() uint64 { return 0 }
