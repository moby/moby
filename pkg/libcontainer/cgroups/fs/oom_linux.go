// +build linux

package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dotcloud/docker/pkg/libcontainer/cgroups"
)

// NotifyOnOOM sends OOM notifications on ch when the cgroup reaches its memory
// limit. If the returned error is not nil, no values will be sent.
func NotifyOnOOM(c *cgroups.Cgroup, ch chan struct{}) error {
	d, err := getCgroupData(c, 0)
	if err != nil {
		return err
	}

	return notifyOnOOM(d, ch)
}

func notifyOnOOM(d *data, ch chan struct{}) error {
	dir, err := d.path("memory")
	if err != nil {
		return err
	}

	fd, _, syserr := syscall.RawSyscall(syscall.SYS_EVENTFD2, 0, syscall.FD_CLOEXEC, 0)
	if syserr != 0 {
		return syserr
	}

	eventfd := os.NewFile(fd, "eventfd")

	oomControl, err := os.Open(filepath.Join(dir, "memory.oom_control"))
	if err != nil {
		eventfd.Close()
		return err
	}

	var (
		eventControlPath = filepath.Join(dir, "cgroup.event_control")

		data = fmt.Sprintf("%d %d", eventfd.Fd(), oomControl.Fd())
	)

	if err := writeFile(dir, "cgroup.event_control", data); err != nil {
		eventfd.Close()
		oomControl.Close()
		return err
	}

	go func() {
		defer eventfd.Close()
		defer oomControl.Close()

		var buf = make([]byte, 8)

		for {
			if _, err := eventfd.Read(buf); err != nil {
				return
			}

			// When a cgroup is destroyed, an event is sent to eventfd.
			// So if the control path is gone, return instead of notifying.
			if _, err := os.Lstat(eventControlPath); os.IsNotExist(err) {
				return
			}

			ch <- struct{}{}
		}
	}()

	return nil
}
