package daemon

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	winio "github.com/Microsoft/go-winio"
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/system"
)

func (d *Daemon) setupDumpStackTrap(root string) {
	// Windows does not support signals like *nix systems. So instead of
	// trapping on SIGUSR1 to dump stacks, we wait on a Win32 event to be
	// signaled. ACL'd to builtin administrators and local system
	ev := "Global\\docker-daemon-" + fmt.Sprint(os.Getpid())
	sd, err := winio.SddlToSecurityDescriptor("D:P(A;;GA;;;BA)(A;;GA;;;SY)")
	if err != nil {
		logrus.Errorf("failed to get security descriptor for debug stackdump event %s: %s", ev, err.Error())
		return
	}
	var sa syscall.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	sa.SecurityDescriptor = uintptr(unsafe.Pointer(&sd[0]))
	h, err := system.CreateEvent(&sa, false, false, ev)
	if h == 0 || err != nil {
		logrus.Errorf("failed to create debug stackdump event %s: %s", ev, err.Error())
		return
	}
	go func() {
		logrus.Debugf("Stackdump - waiting signal at %s", ev)
		for {
			syscall.WaitForSingleObject(h, syscall.INFINITE)
			path, err := signal.DumpStacks(root)
			if err != nil {
				logrus.WithError(err).Error("failed to write goroutines dump")
			} else {
				logrus.Infof("goroutine stacks written to %s", path)
			}
			path, err = d.dumpDaemon(root)
			if err != nil {
				logrus.WithError(err).Error("failed to write daemon datastructure dump")
			} else {
				logrus.Infof("daemon datastructure dump written to %s", path)
			}
		}
	}()
}
