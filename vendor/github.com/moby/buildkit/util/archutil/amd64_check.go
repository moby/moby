//go:build !amd64
// +build !amd64

package archutil

func amd64Supported() (string, error) {
	return check("amd64", Binaryamd64)
}
