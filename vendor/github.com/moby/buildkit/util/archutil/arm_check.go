//go:build !arm

package archutil

func armSupported() (string, error) {
	return check("arm", Binaryarm)
}
