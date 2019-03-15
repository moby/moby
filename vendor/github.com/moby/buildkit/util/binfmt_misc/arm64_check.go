// +build !arm64

package binfmt_misc

func arm64Supported() bool {
	return check(Binaryarm64) == nil
}
