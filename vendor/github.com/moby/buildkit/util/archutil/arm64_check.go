//go:build !arm64
// +build !arm64

package archutil

func arm64Supported() error {
	return check(Binaryarm64)
}
