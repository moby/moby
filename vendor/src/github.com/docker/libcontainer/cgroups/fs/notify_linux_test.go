// +build linux

package fs

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"testing"
	"time"
)

func TestNotifyOnOOM(t *testing.T) {
	helper := NewCgroupTestUtil("memory", t)
	defer helper.cleanup()

	helper.writeFileContents(map[string]string{
		"memory.oom_control":   "",
		"cgroup.event_control": "",
	})

	var eventFd, oomControlFd int

	ooms, err := notifyOnOOM(helper.CgroupData)
	if err != nil {
		t.Fatal("expected no error, got:", err)
	}

	memoryPath, _ := helper.CgroupData.path("memory")
	data, err := readFile(memoryPath, "cgroup.event_control")
	if err != nil {
		t.Fatal("couldn't read event control file:", err)
	}

	if _, err := fmt.Sscanf(data, "%d %d", &eventFd, &oomControlFd); err != nil {
		t.Fatalf("invalid control data %q: %s", data, err)
	}

	// re-open the eventfd
	efd, err := syscall.Dup(eventFd)
	if err != nil {
		t.Fatal("unable to reopen eventfd:", err)
	}
	defer syscall.Close(efd)

	if err != nil {
		t.Fatal("unable to dup event fd:", err)
	}

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, 1)

	if _, err := syscall.Write(efd, buf); err != nil {
		t.Fatal("unable to write to eventfd:", err)
	}

	select {
	case <-ooms:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("no notification on oom channel after 100ms")
	}

	// simulate what happens when a cgroup is destroyed by cleaning up and then
	// writing to the eventfd.
	helper.cleanup()
	if _, err := syscall.Write(efd, buf); err != nil {
		t.Fatal("unable to write to eventfd:", err)
	}

	// give things a moment to shut down
	select {
	case _, ok := <-ooms:
		if ok {
			t.Fatal("expected no oom to be triggered")
		}
	case <-time.After(100 * time.Millisecond):
	}

	if _, _, err := syscall.Syscall(syscall.SYS_FCNTL, uintptr(oomControlFd), syscall.F_GETFD, 0); err != syscall.EBADF {
		t.Error("expected oom control to be closed")
	}

	if _, _, err := syscall.Syscall(syscall.SYS_FCNTL, uintptr(eventFd), syscall.F_GETFD, 0); err != syscall.EBADF {
		t.Error("expected event fd to be closed")
	}
}
