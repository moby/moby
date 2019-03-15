// +build !arm

package binfmt_misc

func armSupported() bool {
	return check(Binaryarm) == nil
}
