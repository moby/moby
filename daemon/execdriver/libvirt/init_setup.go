// +build linux

package libvirt

import (
	"github.com/syndtr/gocapability/capability"
	"io/ioutil"
	"os"
	"path"
)

func setupHostname(args *InitArgs) error {
	hostname := args.GetEnv("HOSTNAME")
	if hostname == "" {
		return nil
	}
	return setHostname(hostname)
}

// Enable device access for privileged containers when using libvirt-lxc
func setupCgroups(args *InitArgs) error {
	if !args.Privileged {
		return nil
	}

	// Enable device access for libvirt-lxc
	devicesCgroupPath := "/sys/fs/cgroup/devices"
	allowFile := path.Join(devicesCgroupPath, "devices.allow")

	if err := ioutil.WriteFile(allowFile, []byte("a *:* rwm"), 0); err != nil {
		return err
	}

	return nil
}

func setupCapabilities(args *InitArgs) error {

	if args.Privileged {
		return nil
	}

	drop := []capability.Cap{
		capability.CAP_SETPCAP,
		capability.CAP_SYS_MODULE,
		capability.CAP_SYS_RAWIO,
		capability.CAP_SYS_PACCT,
		capability.CAP_SYS_ADMIN,
		capability.CAP_SYS_NICE,
		capability.CAP_SYS_RESOURCE,
		capability.CAP_SYS_TIME,
		capability.CAP_SYS_TTY_CONFIG,
		capability.CAP_MKNOD,
		capability.CAP_AUDIT_WRITE,
		capability.CAP_AUDIT_CONTROL,
		capability.CAP_MAC_OVERRIDE,
		capability.CAP_MAC_ADMIN,
	}

	c, err := capability.NewPid(os.Getpid())
	if err != nil {
		return err
	}

	c.Unset(capability.CAPS|capability.BOUNDS, drop...)

	if err := c.Apply(capability.CAPS | capability.BOUNDS); err != nil {
		return err
	}
	return nil
}
