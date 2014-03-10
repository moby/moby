// +build !linux,!darwin,!freebsd

package signal

import (
	"syscall"
)

var signalMap = map[string]syscall.Signal{}
