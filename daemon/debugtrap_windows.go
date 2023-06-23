package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"os"
	"unsafe"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/pkg/stack"
	"golang.org/x/sys/windows"
)

func (daemon *Daemon) setupDumpStackTrap(root string) {
	// Windows does not support signals like *nix systems. So instead of
	// trapping on SIGUSR1 to dump stacks, we wait on a Win32 event to be
	// signaled. ACL'd to builtin administrators and local system
	event := "Global\\stackdump-" + fmt.Sprint(os.Getpid())
	ev, _ := windows.UTF16PtrFromString(event)
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;BA)(A;;GA;;;SY)")
	if err != nil {
		log.G(context.TODO()).Errorf("failed to get security descriptor for debug stackdump event %s: %s", event, err.Error())
		return
	}
	var sa windows.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	sa.SecurityDescriptor = sd
	h, err := windows.CreateEvent(&sa, 0, 0, ev)
	if h == 0 || err != nil {
		log.G(context.TODO()).Errorf("failed to create debug stackdump event %s: %s", event, err.Error())
		return
	}
	go func() {
		log.G(context.TODO()).Debugf("Stackdump - waiting signal at %s", event)
		for {
			windows.WaitForSingleObject(h, windows.INFINITE)
			path, err := stack.DumpToFile(root)
			if err != nil {
				log.G(context.TODO()).WithError(err).Error("failed to write goroutines dump")
			} else {
				log.G(context.TODO()).Infof("goroutine stacks written to %s", path)
			}
		}
	}()
}
