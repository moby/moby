package netnsutils

import (
	"errors"
	"syscall"
	"testing"

	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

// AssertSocketSameNetNS makes a best-effort attempt to assert that conn is in
// the same network namespace as the current goroutine's thread.
func AssertSocketSameNetNS(tb testing.TB, conn syscall.Conn) {
	tb.Helper()

	sc, err := conn.SyscallConn()
	assert.NilError(tb, err)
	sc.Control(func(fd uintptr) {
		srvnsfd, err := unix.IoctlRetInt(int(fd), unix.SIOCGSKNS)
		if err != nil {
			if errors.Is(err, unix.EPERM) {
				tb.Log("Cannot determine socket's network namespace. Do we have CAP_NET_ADMIN?")
				return
			}
			if errors.Is(err, unix.ENOSYS) {
				tb.Log("Cannot query socket's network namespace due to missing kernel support.")
				return
			}
			tb.Fatal(err)
		}
		srvns := netns.NsHandle(srvnsfd)
		defer srvns.Close()

		curns, err := netns.Get()
		assert.NilError(tb, err)
		defer curns.Close()
		if !srvns.Equal(curns) {
			tb.Fatalf("Socket is in network namespace %s, but test goroutine is in %s", srvns, curns)
		}
	})
}
