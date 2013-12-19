package sysinit

import (
	"syscall"
)

func setHostname(hostname string) error {
	return syscall.Sethostname([]byte(hostname))
}
