//go:build !mips64
// +build !mips64

package archutil

func mips64Supported() (string, error) {
	return check("mips64", Binarymips64)
}
