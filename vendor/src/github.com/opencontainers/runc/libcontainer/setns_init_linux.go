// +build linux

package libcontainer

import (
	"fmt"
	"os"
	"syscall"

	"github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/opencontainers/runc/libcontainer/label"
	"github.com/opencontainers/runc/libcontainer/system"
)

// linuxSetnsInit performs the container's initialization for running a new process
// inside an existing container.
type linuxSetnsInit struct {
	config *initConfig
}

func (l *linuxSetnsInit) Init() error {
	if err := setupRlimits(l.config.Config); err != nil {
		return err
	}
	if err := finalizeNamespace(l.config); err != nil {
		return err
	}
	if err := apparmor.ApplyProfile(l.config.Config.AppArmorProfile); err != nil {
		return err
	}
	if l.config.Config.ProcessLabel != "" {
		if err := label.SetProcessLabel(l.config.Config.ProcessLabel); err != nil {
			return err
		}
	}

	args := l.config.Args

	if !l.config.Syscall {
		return system.Execv(args[0], args[0:], os.Environ())
	}

	switch args[0] {
	case "mount":
		// args: mount -t <type> /dev/<device> <mount_point>
		if len(args) != 5 || args[1] != "-t" {
			return fmt.Errorf("syscall invalid format: %v", args)
		}
		fstype := args[2]
		devname := args[3]
		path := args[4]
		if err := syscall.Mount(devname, path, fstype, syscall.MS_MGC_VAL, ""); err != nil {
			return err
		}
	case "umount":
		// args: umount <mount_point>
		if len(l.config.Args) != 2 {
			return fmt.Errorf("syscall invalid format: %v", args)
		}
		path := args[1]
		if err := syscall.Unmount(path, syscall.MNT_DETACH); err != nil {
			return err
		}
	case "mkdir":
		// args: mkdir <path>
		if len(l.config.Args) != 2 {
			return fmt.Errorf("syscall invalid format: %v", args)
		}
		path := args[1]

		st := syscall.Stat_t{}
		err := syscall.Stat(path, &st)
		if err == nil && st.Mode&syscall.S_IFDIR != 0 {
			break
		}

		var mode uint32 = syscall.S_IRUSR | syscall.S_IWUSR | syscall.S_IXUSR
		if err = syscall.Mkdir(path, mode); err != nil {
			return err
		}
	default:
		return fmt.Errorf("syscall %s is not implemented", args[0])
	}

	syscall.Exit(0)
	panic("unreachable")
}
