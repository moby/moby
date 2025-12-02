//go:build !plan9 && !js && !tinygo

package sock

import "syscall"

const (
	SHUT_RD   = syscall.SHUT_RD
	SHUT_RDWR = syscall.SHUT_RDWR
	SHUT_WR   = syscall.SHUT_WR
)
