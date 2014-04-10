package restrict

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/system"
	"path/filepath"
	"syscall"
)

const flags = syscall.MS_BIND | syscall.MS_REC | syscall.MS_RDONLY

var restrictions = map[string]string{
	// dirs
	"/proc/sys":  "",
	"/proc/irq":  "",
	"/proc/acpi": "",

	// files
	"/proc/sysrq-trigger": "/dev/null",
	"/proc/kcore":         "/dev/null",
}

// Restrict locks down access to many areas of proc
// by using the asumption that the user does not have mount caps to
// revert the changes made here
func Restrict(rootfs, empty string) error {
	for dest, source := range restrictions {
		dest = filepath.Join(rootfs, dest)

		// we don't have a "/dev/null" for dirs so have the requester pass a dir
		// for us to bind mount
		switch source {
		case "":
			source = empty
		default:
			source = filepath.Join(rootfs, source)
		}
		if err := system.Mount(source, dest, "bind", flags, ""); err != nil {
			return fmt.Errorf("unable to mount %s over %s %s", source, dest, err)
		}
		if err := system.Mount("", dest, "bind", flags|syscall.MS_REMOUNT, ""); err != nil {
			return fmt.Errorf("unable to mount %s over %s %s", source, dest, err)
		}
	}
	return nil
}
