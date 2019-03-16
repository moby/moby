// +build !amd64

package binfmt_misc

func amd64Supported() bool {
	return check(Binaryamd64) == nil
}
