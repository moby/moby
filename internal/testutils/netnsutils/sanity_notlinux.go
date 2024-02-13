//go:build !linux

package netnsutils

import (
	"syscall"
	"testing"
)

// AssertSocketSameNetNS is a no-op on platforms other than Linux.
func AssertSocketSameNetNS(t testing.TB, conn syscall.Conn) {}
