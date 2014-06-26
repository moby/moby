// +build linux

package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/docker/libcontainer/cgroups"
)

// NotifyOnOOM sends signals on the returned channel when the cgroup reaches
// its memory limit. The channel is closed when the cgroup is removed.
func NotifyOnOOM(c *cgroups.Cgroup) (<-chan struct{}, error) {
	d, err := getCgroupData(c, 0)
	if err != nil {
		return nil, err
	}

	return notifyOnOOM(d)
}

func notifyOnOOM(d *data) (<-chan struct{}, error) {
	dir, err := d.path("memory")
	if err != nil {
		return nil, err
	}

	fd, _, syserr := syscall.RawSyscall(syscall.SYS_EVENTFD2, 0, syscall.FD_CLOEXEC, 0)
	if syserr != 0 {
		return nil, syserr
	}

	eventfd := os.NewFile(fd, "eventfd")

	oomControl, err := os.Open(filepath.Join(dir, "memory.oom_control"))
	if err != nil {
		eventfd.Close()
		return nil, err
	}

	var (
		eventControlPath = filepath.Join(dir, "cgroup.event_control")
		data             = fmt.Sprintf("%d %d", eventfd.Fd(), oomControl.Fd())
	)

	if err := writeFile(dir, "cgroup.event_control", data); err != nil {
		eventfd.Close()
		oomControl.Close()
		return nil, err
	}

	ch := make(chan struct{})

	go func() {
		defer func() {
			close(ch)
			eventfd.Close()
			oomControl.Close()
		}()

		buf := make([]byte, 8)

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

	return ch, nil
}
