// +build !dfsecrets,!dfextall

package instructions

func isSecretMountsSupported() bool {
	return false
}
