//go:build !arm
// +build !arm

package archutil

func armSupported() error {
	return check(Binaryarm)
}
