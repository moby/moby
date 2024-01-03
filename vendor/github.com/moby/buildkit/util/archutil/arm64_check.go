//go:build !arm64
// +build !arm64

package archutil

func arm64Supported() (string, error) {
	return check("arm64", Binaryarm64)
}
