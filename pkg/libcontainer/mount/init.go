// +build linux

package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dotcloud/docker/pkg/label"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer/mount/nodes"
	"github.com/dotcloud/docker/pkg/symlink"
	"github.com/dotcloud/docker/pkg/system"
)

// default mount point flags
const defaultMountFlags = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV

type mount struct {
	source string
	path   string
	device string
	flags  int
	data   string
}

// InitializeMountNamespace setups up the devices, mount points, and filesystems for use inside a
// new mount namepsace
func InitializeMountNamespace(rootfs, console string, container *libcontainer.Container) error {
	var (
		err  error
		flag = syscall.MS_PRIVATE
	)
	if container.NoPivotRoot {
		flag = syscall.MS_SLAVE
	}
	if err := system.Mount("", "/", "", uintptr(flag|syscall.MS_REC), ""); err != nil {
		return fmt.Errorf("mounting / with flags %X %s", (flag | syscall.MS_REC), err)
	}
	if err := system.Mount(rootfs, rootfs, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mouting %s as bind %s", rootfs, err)
	}
	if err := mountSystem(rootfs, container); err != nil {
		return fmt.Errorf("mount system %s", err)
	}
	if err := setupBindmounts(rootfs, container.Mounts); err != nil {
		return fmt.Errorf("bind mounts %s", err)
	}
	if err := nodes.CreateDeviceNodes(rootfs, container.DeviceNodes); err != nil {
		return fmt.Errorf("create device nodes %s", err)
	}
	if err := SetupPtmx(rootfs, console, container.Context["mount_label"]); err != nil {
		return err
	}
	if err := setupDevSymlinks(rootfs); err != nil {
		return fmt.Errorf("dev symlinks %s", err)
	}
	if err := system.Chdir(rootfs); err != nil {
		return fmt.Errorf("chdir into %s %s", rootfs, err)
	}

	if container.NoPivotRoot {
		err = MsMoveRoot(rootfs)
	} else {
		err = PivotRoot(rootfs)
	}
	if err != nil {
		return err
	}

	if container.ReadonlyFs {
		if err := SetReadonly(); err != nil {
			return fmt.Errorf("set readonly %s", err)
		}
	}

	system.Umask(0022)

	return nil
}

// mountSystem sets up linux specific system mounts like sys, proc, shm, and devpts
// inside the mount namespace
func mountSystem(rootfs string, container *libcontainer.Container) error {
	for _, m := range newSystemMounts(rootfs, container.Context["mount_label"], container.Mounts) {
		if err := os.MkdirAll(m.path, 0755); err != nil && !os.IsExist(err) {
			return fmt.Errorf("mkdirall %s %s", m.path, err)
		}
		if err := system.Mount(m.source, m.path, m.device, uintptr(m.flags), m.data); err != nil {
			return fmt.Errorf("mounting %s into %s %s", m.source, m.path, err)
		}
	}

	// Mount all cgroup subsystems into the container, read-only. Create symlinks for each
	// subsystem if any subsystems are merged
	cgroupMounts, err := cgroups.GetMounts()
	if err != nil {
		return err
	}

	cgroupsDir := filepath.Join(rootfs, "/sys/fs/cgroup")

	for _, m := range cgroupMounts {
		dir := filepath.Base(m.Mountpoint)
		mountpoint := filepath.Join(cgroupsDir, dir)

		if err := os.MkdirAll(mountpoint, 0755); err != nil && !os.IsExist(err) {
			return fmt.Errorf("mkdirall %s %s", mountpoint, err)
		}

		// Bind-mount the cgroup to /sys/fs/cgroup with the same name as the outer mount
		if err := system.Mount(m.Mountpoint, mountpoint, "bind", uintptr(syscall.MS_BIND|defaultMountFlags), ""); err != nil {
			return fmt.Errorf("mounting %s into %s %s", m.Mountpoint, mountpoint, err)
		}
		// Make it read-only
		if err := system.Mount(mountpoint, mountpoint, "bind", uintptr(syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|defaultMountFlags), ""); err != nil {
			return fmt.Errorf("remounting %s into %s %s", mountpoint, mountpoint, err)
		}

		hasName := false
		for _, subsys := range m.Subsystems {
			isName := strings.HasPrefix(subsys, "name=")
			canonicalName := subsys
			if isName {
				hasName = true
				canonicalName = subsys[5:]
			}

			// For the merged case dir will be something like "cpu,cpuacct", so
			// we make symlinks for all the pure subsystem names "cpu -> cpu,cpuacct", etc
			if canonicalName != dir {
				if err := os.Symlink(dir, filepath.Join(cgroupsDir, canonicalName)); err != nil {
					return fmt.Errorf("creating cgroup symlink for %s: %s", dir, err)
				}
			}
		}

		// For named cgroups, such as name=systemd we mount a read-write subset at the
		// current cgroup path. This lets e.g. systemd work inside a container, as it can create subcgroups inside the
		// current cgroup, while not being able to do anything dangerous in the real cgroups
		if hasName {
			cgroupPath, _ := m.GetThisCgroupDir()
			if cgroupPath != "" && cgroupPath != "/" {
				if err := system.Mount(filepath.Join(m.Mountpoint, cgroupPath), filepath.Join(mountpoint, cgroupPath), "bind", uintptr(syscall.MS_BIND|defaultMountFlags), ""); err != nil {
					return fmt.Errorf("mounting %s into %s %s", filepath.Join(m.Mountpoint, cgroupPath), filepath.Join(mountpoint, cgroupPath), err)
				}
			}
		}
	}

	// Make /sys/fs/cgroup read-only
	if err := system.Mount(cgroupsDir, cgroupsDir, "bind", uintptr(syscall.MS_REMOUNT|syscall.MS_RDONLY|defaultMountFlags), ""); err != nil {
		return fmt.Errorf("remounting %s read-only %s", cgroupsDir, err)
	}

	return nil
}

func createIfNotExists(path string, isDir bool) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if isDir {
				if err := os.MkdirAll(path, 0755); err != nil {
					return err
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					return err
				}
				f, err := os.OpenFile(path, os.O_CREATE, 0755)
				if err != nil {
					return err
				}
				f.Close()
			}
		}
	}
	return nil
}

func setupDevSymlinks(rootfs string) error {
	var links = [][2]string{
		{"/proc/self/fd", "/dev/fd"},
		{"/proc/self/fd/0", "/dev/stdin"},
		{"/proc/self/fd/1", "/dev/stdout"},
		{"/proc/self/fd/2", "/dev/stderr"},
	}

	// kcore support can be toggled with CONFIG_PROC_KCORE; only create a symlink
	// in /dev if it exists in /proc.
	if _, err := os.Stat("/proc/kcore"); err == nil {
		links = append(links, [2]string{"/proc/kcore", "/dev/kcore"})
	}

	for _, link := range links {
		var (
			src = link[0]
			dst = filepath.Join(rootfs, link[1])
		)

		if err := os.Symlink(src, dst); err != nil && !os.IsExist(err) {
			return fmt.Errorf("symlink %s %s %s", src, dst, err)
		}
	}

	return nil
}

func setupBindmounts(rootfs string, bindMounts libcontainer.Mounts) error {
	for _, m := range bindMounts.OfType("bind") {
		var (
			flags = syscall.MS_BIND | syscall.MS_REC
			dest  = filepath.Join(rootfs, m.Destination)
		)
		if !m.Writable {
			flags = flags | syscall.MS_RDONLY
		}

		stat, err := os.Stat(m.Source)
		if err != nil {
			return err
		}

		dest, err = symlink.FollowSymlinkInScope(dest, rootfs)
		if err != nil {
			return err
		}

		if err := createIfNotExists(dest, stat.IsDir()); err != nil {
			return fmt.Errorf("Creating new bind-mount target, %s", err)
		}

		if err := system.Mount(m.Source, dest, "bind", uintptr(flags), ""); err != nil {
			return fmt.Errorf("mounting %s into %s %s", m.Source, dest, err)
		}
		if !m.Writable {
			if err := system.Mount(m.Source, dest, "bind", uintptr(flags|syscall.MS_REMOUNT), ""); err != nil {
				return fmt.Errorf("remounting %s into %s %s", m.Source, dest, err)
			}
		}
		if m.Private {
			if err := system.Mount("", dest, "none", uintptr(syscall.MS_PRIVATE), ""); err != nil {
				return fmt.Errorf("mounting %s private %s", dest, err)
			}
		}
	}
	return nil
}

// TODO: this is crappy right now and should be cleaned up with a better way of handling system and
// standard bind mounts allowing them to be more dynamic
func newSystemMounts(rootfs, mountLabel string, mounts libcontainer.Mounts) []mount {
	systemMounts := []mount{
		{source: "proc", path: filepath.Join(rootfs, "proc"), device: "proc", flags: defaultMountFlags},
		{source: "sysfs", path: filepath.Join(rootfs, "sys"), device: "sysfs", flags: defaultMountFlags},
		{source: "tmpfs", path: filepath.Join(rootfs, "dev"), device: "tmpfs", flags: syscall.MS_NOSUID | syscall.MS_STRICTATIME, data: label.FormatMountLabel("mode=755", mountLabel)},
		{source: "tmpfs", path: filepath.Join(rootfs, "sys/fs/cgroup"), device: "tmpfs", flags: defaultMountFlags},
		{source: "shm", path: filepath.Join(rootfs, "dev", "shm"), device: "tmpfs", flags: defaultMountFlags, data: label.FormatMountLabel("mode=1777,size=65536k", mountLabel)},
		{source: "devpts", path: filepath.Join(rootfs, "dev", "pts"), device: "devpts", flags: syscall.MS_NOSUID | syscall.MS_NOEXEC, data: label.FormatMountLabel("newinstance,ptmxmode=0666,mode=620,gid=5", mountLabel)},
	}

	return systemMounts
}
