//go:build !windows

package oci

// no effect for non-Windows
func normalizeMountType(mType string) string {
	return mType
}
