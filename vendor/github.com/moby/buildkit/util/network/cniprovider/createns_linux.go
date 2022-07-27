//go:build linux
// +build linux

package cniprovider

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/containerd/containerd/oci"
	"github.com/moby/buildkit/util/bklog"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func cleanOldNamespaces(c *cniProvider) {
	nsDir := filepath.Join(c.root, "net/cni")
	dirEntries, err := os.ReadDir(nsDir)
	if err != nil {
		bklog.L.Debugf("could not read %q for cleanup: %s", nsDir, err)
		return
	}
	go func() {
		for _, d := range dirEntries {
			id := d.Name()
			ns := cniNS{
				id:       id,
				nativeID: filepath.Join(c.root, "net/cni", id),
				handle:   c.CNI,
			}
			if err := ns.release(); err != nil {
				bklog.L.Warningf("failed to release network namespace %q left over from previous run: %s", id, err)
			}
		}
	}()
}

func createNetNS(c *cniProvider, id string) (string, error) {
	nsPath := filepath.Join(c.root, "net/cni", id)
	if err := os.MkdirAll(filepath.Dir(nsPath), 0700); err != nil {
		return "", err
	}

	f, err := os.Create(nsPath)
	if err != nil {
		deleteNetNS(nsPath)
		return "", err
	}
	if err := f.Close(); err != nil {
		deleteNetNS(nsPath)
		return "", err
	}
	procNetNSBytes, err := syscall.BytePtrFromString("/proc/self/ns/net")
	if err != nil {
		deleteNetNS(nsPath)
		return "", err
	}
	nsPathBytes, err := syscall.BytePtrFromString(nsPath)
	if err != nil {
		deleteNetNS(nsPath)
		return "", err
	}
	beforeFork()

	pid, _, errno := syscall.RawSyscall6(syscall.SYS_CLONE, uintptr(syscall.SIGCHLD)|unix.CLONE_NEWNET, 0, 0, 0, 0, 0)
	if errno != 0 {
		afterFork()
		deleteNetNS(nsPath)
		return "", errno
	}

	if pid != 0 {
		afterFork()
		var ws unix.WaitStatus
		_, err = unix.Wait4(int(pid), &ws, 0, nil)
		for err == syscall.EINTR {
			_, err = unix.Wait4(int(pid), &ws, 0, nil)
		}

		if err != nil {
			deleteNetNS(nsPath)
			return "", errors.Wrapf(err, "failed to find pid=%d process", pid)
		}
		errno = syscall.Errno(ws.ExitStatus())
		if errno != 0 {
			deleteNetNS(nsPath)
			return "", errors.Wrapf(errno, "failed to mount %s (pid=%d)", nsPath, pid)
		}
		return nsPath, nil
	}
	afterForkInChild()
	_, _, errno = syscall.RawSyscall6(syscall.SYS_MOUNT, uintptr(unsafe.Pointer(procNetNSBytes)), uintptr(unsafe.Pointer(nsPathBytes)), 0, uintptr(unix.MS_BIND), 0, 0)
	syscall.RawSyscall(syscall.SYS_EXIT, uintptr(errno), 0, 0)
	panic("unreachable")
}

func setNetNS(s *specs.Spec, nsPath string) error {
	return oci.WithLinuxNamespace(specs.LinuxNamespace{
		Type: specs.NetworkNamespace,
		Path: nsPath,
	})(nil, nil, nil, s)
}

func unmountNetNS(nsPath string) error {
	if err := unix.Unmount(nsPath, unix.MNT_DETACH); err != nil {
		if err != syscall.EINVAL && err != syscall.ENOENT {
			return errors.Wrap(err, "error unmounting network namespace")
		}
	}
	return nil
}

func deleteNetNS(nsPath string) error {
	if err := os.Remove(nsPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Wrapf(err, "error removing network namespace %s", nsPath)
	}
	return nil
}
