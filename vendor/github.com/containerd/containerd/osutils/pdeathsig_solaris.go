// +build solaris

package osutils

// SetPDeathSig is a no-op on Solaris as Pdeathsig is not defined.
func SetPDeathSig() *syscall.SysProcAttr {
	return nil
}
