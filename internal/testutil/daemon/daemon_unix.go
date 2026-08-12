//go:build !windows

package daemon

import (
	"net/http"
	"os/exec"
	"strconv"
	"syscall"
	"testing"

	"github.com/moby/sys/mount"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const (
	defaultContainerdSocket      = "/var/run/docker/containerd/containerd.sock"
	defaultDockerdRootlessBinary = "dockerd-rootless.sh"
	defaultUnixSocket            = "/var/run/docker.sock"
)

func (d *Daemon) rootlessCommand(dockerdBinary string) (string, []string, error) {
	if d.rootlessUser == nil {
		return dockerdBinary, nil, nil
	}
	if d.dockerdBinary != defaultDockerdBinary {
		return "", nil, errors.Errorf("[%s] DOCKER_ROOTLESS doesn't support non-default dockerd binary path %q", d.id, d.dockerdBinary)
	}
	return "sudo", []string{
		"-u", d.rootlessUser.Username,
		"--preserve-env",
		"--preserve-env=PATH", // Pass through PATH, overriding secure_path.
		"XDG_RUNTIME_DIR=" + d.rootlessXDGRuntimeDir,
		"HOME=" + d.rootlessUser.HomeDir,
		"--",
		defaultDockerdRootlessBinary,
	}, nil
}

func (d *Daemon) platformArgs() []string {
	return []string{"--userland-proxy=" + strconv.FormatBool(d.userlandProxy)}
}

func defaultHostConfig() (*http.Transport, string, string, string) {
	return &http.Transport{}, "http", "unix", defaultUnixSocket
}

// cleanupMount unmounts the daemon root directory, or logs a message if
// unmounting failed.
func cleanupMount(t testing.TB, d *Daemon) {
	t.Helper()
	if err := mount.Unmount(d.Root); err != nil {
		d.log.Logf("[%s] unable to unmount daemon root (%s): %v", d.id, d.Root, err)
	}
}

// SignalDaemonDump sends a signal to the daemon to write a dump file
func SignalDaemonDump(pid int) {
	_ = unix.Kill(pid, unix.SIGQUIT)
}

func signalDaemonReload(pid int) error {
	return unix.Kill(pid, unix.SIGHUP)
}

func setsid(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}
