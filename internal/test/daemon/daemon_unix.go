// +build !windows

package daemon // import "github.com/docker/docker/internal/test/daemon"

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/containerd/go-cni"
	"golang.org/x/sys/unix"
	"gotest.tools/assert"
)

// CgroupNamespace returns the cgroup namespace the daemon is running in
func (d *Daemon) CgroupNamespace(t assert.TestingT) string {
	link, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/cgroup", d.Pid()))
	assert.NilError(t, err)

	return strings.TrimSpace(link)
}

// SignalDaemonDump sends a signal to the daemon to write a dump file
func SignalDaemonDump(pid int) {
	unix.Kill(pid, unix.SIGQUIT)
}

func signalDaemonReload(pid int) error {
	return unix.Kill(pid, unix.SIGHUP)
}

var daemonNetworks cni.CNI
var daemonNetworksOnce sync.Once
