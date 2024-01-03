//go:build !mips64le
// +build !mips64le

package archutil

func mips64leSupported() (string, error) {
	return check("mips64le", Binarymips64le)
}
