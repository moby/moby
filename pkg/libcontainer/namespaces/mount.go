package namespaces

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/system"
	"log"
	"os"
	"path/filepath"
	"syscall"
)

var (
	// default mount point options
	defaults = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
)

func SetupNewMountNamespace(rootfs, console string, readonly bool) error {
	if err := system.Mount("", "/", "", syscall.MS_SLAVE|syscall.MS_REC, ""); err != nil {
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

	ptmx := filepath.Join(rootfs, "dev/ptmx")
	if err := os.Remove(ptmx); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink(filepath.Join(rootfs, "pts/ptmx"), ptmx); err != nil {
		return fmt.Errorf("symlink dev ptmx %s", err)
	}

	if err := setupDev(rootfs); err != nil {
		return err
	}

	if err := setupConsole(rootfs, console); err != nil {
		return err
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
		stat, err := os.Stat(filepath.Join("/dev", node))
		if err != nil {
			return err
		}

		var (
			dest = filepath.Join(rootfs, "dev", node)
			st   = stat.Sys().(*syscall.Stat_t)
		)

		log.Printf("copy %s to %s %d\n", node, dest, st.Rdev)
		if err := system.Mknod(dest, st.Mode, int(st.Rdev)); err != nil && !os.IsExist(err) {
			return fmt.Errorf("copy %s %s", node, err)
		}
	}
	return nil
}

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

func setupConsole(rootfs, console string) error {
	oldMask := system.Umask(0000)
	defer system.Umask(oldMask)

	stat, err := os.Stat(console)
	if err != nil {
		return fmt.Errorf("stat console %s %s", console, err)
	}
	st := stat.Sys().(*syscall.Stat_t)

	dest := filepath.Join(rootfs, "dev/console")
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
		{source: "proc", path: filepath.Join(rootfs, "proc"), device: "proc", flags: defaults},
		{source: "sysfs", path: filepath.Join(rootfs, "sys"), device: "sysfs", flags: defaults},
		{source: "tmpfs", path: filepath.Join(rootfs, "dev"), device: "tmpfs", flags: syscall.MS_NOSUID | syscall.MS_STRICTATIME, data: "mode=755"},
		{source: "shm", path: filepath.Join(rootfs, "dev", "shm"), device: "tmpfs", flags: defaults, data: "mode=1777"},
		{source: "devpts", path: filepath.Join(rootfs, "dev", "pts"), device: "devpts", flags: syscall.MS_NOSUID | syscall.MS_NOEXEC, data: "newinstance,ptmxmode=0666,mode=620,gid=5"},
		{source: "tmpfs", path: filepath.Join(rootfs, "run"), device: "tmpfs", flags: syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_STRICTATIME, data: "mode=755"},
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

func remountProc() error {
	if err := system.Unmount("/proc", syscall.MNT_DETACH); err != nil {
		return err
	}
	if err := system.Mount("proc", "/proc", "proc", uintptr(defaults), ""); err != nil {
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
		if err := system.Mount("sysfs", "/sys", "sysfs", uintptr(defaults), ""); err != nil {
			return err
		}
	}
	return nil
}
