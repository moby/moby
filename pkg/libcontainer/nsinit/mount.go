// +build linux

package nsinit

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/system"
	"os"
	"path/filepath"
	"syscall"
)

// default mount point flags
const defaultMountFlags = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV

// setupNewMountNamespace is used to initialize a new mount namespace for an new
// container in the rootfs that is specified.
//
// There is no need to unmount the new mounts because as soon as the mount namespace
// is no longer in use, the mounts will be removed automatically
func setupNewMountNamespace(rootfs, console string, readonly bool) error {
	// mount as slave so that the new mounts do not propagate to the host
	if err := system.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mounting / as slave %s", err)
	}
	if err := system.Mount(rootfs, rootfs, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mouting %s as bind %s", rootfs, err)
	}
	if readonly {
		if err := system.Mount(rootfs, rootfs, "bind", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_REC, ""); err != nil {
			return fmt.Errorf("mounting %s as readonly %s", rootfs, err)
		}
	}
	if err := mountSystem(rootfs); err != nil {
		return fmt.Errorf("mount system %s", err)
	}
	if err := copyDevNodes(rootfs); err != nil {
		return fmt.Errorf("copy dev nodes %s", err)
	}
	// In non-privileged mode, this fails. Discard the error.
	setupLoopbackDevices(rootfs)
	if err := setupDev(rootfs); err != nil {
		return err
	}
	if console != "" {
		if err := setupPtmx(rootfs, console); err != nil {
			return err
		}
	}
	if err := system.Chdir(rootfs); err != nil {
		return fmt.Errorf("chdir into %s %s", rootfs, err)
	}
	if err := system.Mount(rootfs, "/", "", syscall.MS_MOVE, ""); err != nil {
		return fmt.Errorf("mount move %s into / %s", rootfs, err)
	}
	if err := system.Chroot("."); err != nil {
		return fmt.Errorf("chroot . %s", err)
	}
	if err := system.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %s", err)
	}

	system.Umask(0022)

	return nil
}

// copyDevNodes mknods the hosts devices so the new container has access to them
func copyDevNodes(rootfs string) error {
	oldMask := system.Umask(0000)
	defer system.Umask(oldMask)

	for _, node := range []string{
		"null",
		"zero",
		"full",
		"random",
		"urandom",
		"tty",
	} {
		if err := copyDevNode(rootfs, node); err != nil {
			return err
		}
	}
	return nil
}

func setupLoopbackDevices(rootfs string) error {
	for i := 0; ; i++ {
		if err := copyDevNode(rootfs, fmt.Sprintf("loop%d", i)); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			break
		}

	}
	return nil
}

func copyDevNode(rootfs, node string) error {
	stat, err := os.Stat(filepath.Join("/dev", node))
	if err != nil {
		return err
	}
	var (
		dest = filepath.Join(rootfs, "dev", node)
		st   = stat.Sys().(*syscall.Stat_t)
	)
	if err := system.Mknod(dest, st.Mode, int(st.Rdev)); err != nil && !os.IsExist(err) {
		return fmt.Errorf("copy %s %s", node, err)
	}
	return nil
}

// setupDev symlinks the current processes pipes into the
// appropriate destination on the containers rootfs
func setupDev(rootfs string) error {
	for _, link := range []struct {
		from string
		to   string
	}{
		{"/proc/kcore", "/dev/core"},
		{"/proc/self/fd", "/dev/fd"},
		{"/proc/self/fd/0", "/dev/stdin"},
		{"/proc/self/fd/1", "/dev/stdout"},
		{"/proc/self/fd/2", "/dev/stderr"},
	} {
		dest := filepath.Join(rootfs, link.to)
		if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s %s", dest, err)
		}
		if err := os.Symlink(link.from, dest); err != nil {
			return fmt.Errorf("symlink %s %s", dest, err)
		}
	}
	return nil
}

// setupConsole ensures that the container has a proper /dev/console setup
func setupConsole(rootfs, console string) error {
	oldMask := system.Umask(0000)
	defer system.Umask(oldMask)

	stat, err := os.Stat(console)
	if err != nil {
		return fmt.Errorf("stat console %s %s", console, err)
	}
	var (
		st   = stat.Sys().(*syscall.Stat_t)
		dest = filepath.Join(rootfs, "dev/console")
	)
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s %s", dest, err)
	}
	if err := os.Chmod(console, 0600); err != nil {
		return err
	}
	if err := os.Chown(console, 0, 0); err != nil {
		return err
	}
	if err := system.Mknod(dest, (st.Mode&^07777)|0600, int(st.Rdev)); err != nil {
		return fmt.Errorf("mknod %s %s", dest, err)
	}
	if err := system.Mount(console, dest, "bind", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind %s to %s %s", console, dest, err)
	}
	return nil
}

// mountSystem sets up linux specific system mounts like sys, proc, shm, and devpts
// inside the mount namespace
func mountSystem(rootfs string) error {
	for _, m := range []struct {
		source string
		path   string
		device string
		flags  int
		data   string
	}{
		{source: "proc", path: filepath.Join(rootfs, "proc"), device: "proc", flags: defaultMountFlags},
		{source: "sysfs", path: filepath.Join(rootfs, "sys"), device: "sysfs", flags: defaultMountFlags},
		{source: "shm", path: filepath.Join(rootfs, "dev", "shm"), device: "tmpfs", flags: defaultMountFlags, data: "mode=1777,size=65536k"},
		{source: "devpts", path: filepath.Join(rootfs, "dev", "pts"), device: "devpts", flags: syscall.MS_NOSUID | syscall.MS_NOEXEC, data: "newinstance,ptmxmode=0666,mode=620,gid=5"},
	} {
		if err := os.MkdirAll(m.path, 0755); err != nil && !os.IsExist(err) {
			return fmt.Errorf("mkdirall %s %s", m.path, err)
		}
		if err := system.Mount(m.source, m.path, m.device, uintptr(m.flags), m.data); err != nil {
			return fmt.Errorf("mounting %s into %s %s", m.source, m.path, err)
		}
	}
	return nil
}

// setupPtmx adds a symlink to pts/ptmx for /dev/ptmx and
// finishes setting up /dev/console
func setupPtmx(rootfs, console string) error {
	ptmx := filepath.Join(rootfs, "dev/ptmx")
	if err := os.Remove(ptmx); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink("pts/ptmx", ptmx); err != nil {
		return fmt.Errorf("symlink dev ptmx %s", err)
	}
	if err := setupConsole(rootfs, console); err != nil {
		return err
	}
	return nil
}

// remountProc is used to detach and remount the proc filesystem
// commonly needed with running a new process inside an existing container
func remountProc() error {
	if err := system.Unmount("/proc", syscall.MNT_DETACH); err != nil {
		return err
	}
	if err := system.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), ""); err != nil {
		return err
	}
	return nil
}

func remountSys() error {
	if err := system.Unmount("/sys", syscall.MNT_DETACH); err != nil {
		if err != syscall.EINVAL {
			return err
		}
	} else {
		if err := system.Mount("sysfs", "/sys", "sysfs", uintptr(defaultMountFlags), ""); err != nil {
			return err
		}
	}
	return nil
}
