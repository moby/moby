// +build !linux,!darwin,!freebsd,!windows,!solaris

package daemon

func setupDumpStackTrap(_ string) {
	return
}
