// +build !dfsecrets

package instructions

func isSecretMountsSupported() bool {
	return false
}
