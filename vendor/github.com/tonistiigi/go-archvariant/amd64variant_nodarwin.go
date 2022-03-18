//go:build amd64 && !darwin
// +build amd64,!darwin

package archvariant

func osAVX512Supported(ax uint32) bool {
	return ax&v4OSSupport == v4OSSupport
}
