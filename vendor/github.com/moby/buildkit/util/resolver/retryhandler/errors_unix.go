//go:build !windows

package retryhandler

import "syscall"

const errConnectionReset = syscall.ECONNRESET
