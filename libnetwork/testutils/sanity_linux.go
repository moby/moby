package testutils

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
func AssertSocketSameNetNS(t testing.TB, conn syscall.Conn) {
	t.Helper()

	sc, err := conn.SyscallConn()
	assert.NilError(t, err)
	sc.Control(func(fd uintptr) {
		srvnsfd, err := unix.IoctlRetInt(int(fd), unix.SIOCGSKNS)
		if err != nil {
			if errors.Is(err, unix.EPERM) {
				t.Log("Cannot determine socket's network namespace. Do we have CAP_NET_ADMIN?")
				return
			}
			if errors.Is(err, unix.ENOSYS) {
				t.Log("Cannot query socket's network namespace due to missing kernel support.")
				return
			}
			t.Fatal(err)
		}
		srvns := netns.NsHandle(srvnsfd)
		defer srvns.Close()

		curns, err := netns.Get()
		assert.NilError(t, err)
		defer curns.Close()
		if !srvns.Equal(curns) {
			t.Fatalf("Socket is in network namespace %s, but test goroutine is in %s", srvns, curns)
		}
	})
}
