//go:build !arm
// +build !arm

package archutil

func armSupported() (string, error) {
	return check("arm", Binaryarm)
}
