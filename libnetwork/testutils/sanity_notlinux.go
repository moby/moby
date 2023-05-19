//go:build !linux

package testutils

import (
	"syscall"
	"testing"
)

// AssertSocketSameNetNS is a no-op on platforms other than Linux.
func AssertSocketSameNetNS(t testing.TB, conn syscall.Conn) {}
