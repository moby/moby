package lxc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/docker/libcontainer/netlink"
	"github.com/dotcloud/docker/daemon/execdriver"
	"github.com/dotcloud/docker/pkg/system"
	"github.com/dotcloud/docker/pkg/user"
	"github.com/syndtr/gocapability/capability"
)

// Clear environment pollution introduced by lxc-start
func setupEnv(args *execdriver.InitArgs) error {
	// Get env
	var env []string
	content, err := ioutil.ReadFile(".dockerenv")
	if err != nil {
		return fmt.Errorf("Unable to load environment variables: %v", err)
	}
	if err := json.Unmarshal(content, &env); err != nil {
		return fmt.Errorf("Unable to unmarshal environment variables: %v", err)
	}
	// Propagate the plugin-specific container env variable
	env = append(env, "container="+os.Getenv("container"))

	args.Env = env

	os.Clearenv()
	for _, kv := range args.Env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 1 {
			parts = append(parts, "")
		}
		os.Setenv(parts[0], parts[1])
	}

	return nil
}

func setupHostname(args *execdriver.InitArgs) error {
	hostname := getEnv(args, "HOSTNAME")
	if hostname == "" {
		return nil
	}
	return setHostname(hostname)
}

// Setup networking
func setupNetworking(args *execdriver.InitArgs) error {
	if args.Ip != "" {
		// eth0
		iface, err := net.InterfaceByName("eth0")
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		ip, ipNet, err := net.ParseCIDR(args.Ip)
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		if err := netlink.NetworkLinkAddIp(iface, ip, ipNet); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		if err := netlink.NetworkSetMTU(iface, args.Mtu); err != nil {
			return fmt.Errorf("Unable to set MTU: %v", err)
		}
		if err := netlink.NetworkLinkUp(iface); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}

		// loopback
		iface, err = net.InterfaceByName("lo")
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		if err := netlink.NetworkLinkUp(iface); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
	}
	if args.Gateway != "" {
		gw := net.ParseIP(args.Gateway)
		if gw == nil {
			return fmt.Errorf("Unable to set up networking, %s is not a valid gateway IP", args.Gateway)
		}

		if err := netlink.AddDefaultGw(gw.String(), "eth0"); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
	}

	return nil
}

// Setup working directory
func setupWorkingDirectory(args *execdriver.InitArgs) error {
	if args.WorkDir == "" {
		return nil
	}
	if err := syscall.Chdir(args.WorkDir); err != nil {
		return fmt.Errorf("Unable to change dir to %v: %v", args.WorkDir, err)
	}
	return nil
}

// Takes care of dropping privileges to the desired user
func changeUser(args *execdriver.InitArgs) error {
	uid, gid, suppGids, err := user.GetUserGroupSupplementary(
		args.User,
		syscall.Getuid(), syscall.Getgid(),
	)
	if err != nil {
		return err
	}

	if err := syscall.Setgroups(suppGids); err != nil {
		return fmt.Errorf("Setgroups failed: %v", err)
	}
	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("Setgid failed: %v", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("Setuid failed: %v", err)
	}

	return nil
}

var whiteList = []capability.Cap{
	capability.CAP_MKNOD,
	capability.CAP_SETUID,
	capability.CAP_SETGID,
	capability.CAP_CHOWN,
	capability.CAP_NET_RAW,
	capability.CAP_DAC_OVERRIDE,
	capability.CAP_FOWNER,
	capability.CAP_FSETID,
	capability.CAP_KILL,
	capability.CAP_SETGID,
	capability.CAP_SETUID,
	capability.CAP_LINUX_IMMUTABLE,
	capability.CAP_NET_BIND_SERVICE,
	capability.CAP_NET_BROADCAST,
	capability.CAP_IPC_LOCK,
	capability.CAP_IPC_OWNER,
	capability.CAP_SYS_CHROOT,
	capability.CAP_SYS_PTRACE,
	capability.CAP_SYS_BOOT,
	capability.CAP_LEASE,
	capability.CAP_SETFCAP,
	capability.CAP_WAKE_ALARM,
	capability.CAP_BLOCK_SUSPEND,
}

func dropBoundingSet() error {
	c, err := capability.NewPid(os.Getpid())
	if err != nil {
		return err
	}
	c.Clear(capability.BOUNDS)
	c.Set(capability.BOUNDS, whiteList...)

	if err := c.Apply(capability.BOUNDS); err != nil {
		return err
	}

	return nil
}

const allCapabilityTypes = capability.CAPS | capability.BOUNDS

func dropCapabilities() error {
	c, err := capability.NewPid(os.Getpid())
	if err != nil {
		return err
	}
	c.Clear(allCapabilityTypes)
	c.Set(allCapabilityTypes, whiteList...)

	if err := c.Apply(allCapabilityTypes); err != nil {
		return err
	}

	return nil
}

func setupCapabilities(args *execdriver.InitArgs) error {
	if err := system.CloseFdsFrom(3); err != nil {
		return err
	}

	if !args.Privileged {
		// drop capabilities in bounding set before changing user
		if err := dropBoundingSet(); err != nil {
			return fmt.Errorf("drop bounding set %s", err)
		}

		// preserve existing capabilities while we change users
		if err := system.SetKeepCaps(); err != nil {
			return fmt.Errorf("set keep caps %s", err)
		}
	}

	if err := changeUser(args); err != nil {
		return err
	}

	if !args.Privileged {
		if err := system.ClearKeepCaps(); err != nil {
			return fmt.Errorf("clear keep caps %s", err)
		}

		// drop all other capabilities
		if err := dropCapabilities(); err != nil {
			return fmt.Errorf("drop capabilities %s", err)
		}
	}

	if err := setupWorkingDirectory(args); err != nil {
		return err
	}

	return nil
}

func getEnv(args *execdriver.InitArgs, key string) string {
	for _, kv := range args.Env {
		parts := strings.SplitN(kv, "=", 2)
		if parts[0] == key && len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}
