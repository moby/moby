// +build !mips64

package archutil

func mips64Supported() error {
	return check(Binarymips64)
}
