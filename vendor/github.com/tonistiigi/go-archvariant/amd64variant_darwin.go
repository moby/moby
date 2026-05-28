//go:build amd64 && darwin
// +build amd64,darwin

package archvariant

func darwinSupportsAVX512() bool

func osAVX512Supported(ax uint32) bool {
	return ax&v3OSSupport == v3OSSupport && darwinSupportsAVX512()
}
