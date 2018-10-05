// +build !dfssh,!dfextall

package instructions

func isSSHMountsSupported() bool {
	return false
}
