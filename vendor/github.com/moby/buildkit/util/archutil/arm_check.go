// +build !arm

package archutil

func armSupported() error {
	return check(Binaryarm)
}
