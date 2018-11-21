// +build !dfssh

package instructions

func isSSHMountsSupported() bool {
	return false
}
