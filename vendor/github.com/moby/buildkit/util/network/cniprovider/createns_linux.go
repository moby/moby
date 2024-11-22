//go:build linux

package cniprovider

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

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

// unshareAndMount needs to be called in a separate thread
func unshareAndMountNetNS(target string) error {
	if err := syscall.Unshare(syscall.CLONE_NEWNET); err != nil {
		return err
	}

	return syscall.Mount(fmt.Sprintf("/proc/self/task/%d/ns/net", syscall.Gettid()), target, "", syscall.MS_BIND, "")
}

func createNetNS(c *cniProvider, id string) (_ string, err error) {
	nsPath := filepath.Join(c.root, "net/cni", id)
	if err := os.MkdirAll(filepath.Dir(nsPath), 0700); err != nil {
		return "", err
	}

	f, err := os.Create(nsPath)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			deleteNetNS(nsPath)
		}
	}()
	if err := f.Close(); err != nil {
		return "", err
	}

	errCh := make(chan error)

	go func() {
		defer close(errCh)
		runtime.LockOSThread()

		if err := unshareAndMountNetNS(nsPath); err != nil {
			errCh <- err
		}

		// we leave the thread locked so go runtime terminates the thread
	}()

	if err := <-errCh; err != nil {
		return "", err
	}
	return nsPath, nil
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
