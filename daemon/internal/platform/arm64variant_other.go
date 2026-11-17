//go:build !linux

package platform

func arm64Variant() string {
	return ""
}
