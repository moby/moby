//go:build !amd64
// +build !amd64

package archutil

func amd64Supported() error {
	return check(Binaryamd64)
}
