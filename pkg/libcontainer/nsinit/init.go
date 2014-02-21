// +build linux

package main

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/capabilities"
	"github.com/dotcloud/docker/pkg/libcontainer/network"
	"github.com/dotcloud/docker/pkg/system"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
)

func initCommand(container *libcontainer.Container, console string, pipe io.ReadCloser, args []string) error {
	rootfs, err := resolveRootfs()
	if err != nil {
		return err
	}

	// We always read this as it is a way to sync with the parent as well
	tempVethName, err := getVethName(pipe)
	if err != nil {
		return err
	}

	// close pipes so that we can replace it with the pty
	os.Stdin.Close()
	os.Stdout.Close()
	os.Stderr.Close()

	slave, err := openTerminal(console, syscall.O_RDWR)
	if err != nil {
		return fmt.Errorf("open terminal %s", err)
	}
	if err := dupSlave(slave); err != nil {
		return fmt.Errorf("dup2 slave %s", err)
	}
	if _, err := system.Setsid(); err != nil {
		return fmt.Errorf("setsid %s", err)
	}
	if err := system.Setctty(); err != nil {
		return fmt.Errorf("setctty %s", err)
	}
	if err := system.ParentDeathSignal(); err != nil {
		return fmt.Errorf("parent deth signal %s", err)
	}
	if err := setupNewMountNamespace(rootfs, console, container.ReadonlyFs); err != nil {
		return fmt.Errorf("setup mount namespace %s", err)
	}
	if err := setupVethNetwork(container.Network, tempVethName); err != nil {
		return fmt.Errorf("setup networking %s", err)
	}
	if err := system.Sethostname(container.Hostname); err != nil {
		return fmt.Errorf("sethostname %s", err)
	}
	if err := capabilities.DropCapabilities(container); err != nil {
		return fmt.Errorf("drop capabilities %s", err)
	}
	if err := setupUser(container); err != nil {
		return fmt.Errorf("setup user %s", err)
	}
	if container.WorkingDir != "" {
		if err := system.Chdir(container.WorkingDir); err != nil {
			return fmt.Errorf("chdir to %s %s", container.WorkingDir, err)
		}
	}
	if err := system.Exec(args[0], args[0:], container.Env); err != nil {
		return fmt.Errorf("exec %s", err)
	}
	panic("unreachable")
}

// resolveRootfs ensures that the current working directory is
// not a symlink and returns the absolute path to the rootfs
func resolveRootfs() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	rootfs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(rootfs)
}

func setupUser(container *libcontainer.Container) error {
	// TODO: honor user passed on container
	if err := system.Setgroups(nil); err != nil {
		return err
	}
	if err := system.Setresgid(0, 0, 0); err != nil {
		return err
	}
	if err := system.Setresuid(0, 0, 0); err != nil {
		return err
	}
	return nil
}

// dupSlave dup2 the pty slave's fd into stdout and stdin and ensures that
// the slave's fd is 0, or stdin
func dupSlave(slave *os.File) error {
	if slave.Fd() != 0 {
		return fmt.Errorf("slave fd not 0 %d", slave.Fd())
	}
	if err := system.Dup2(slave.Fd(), 1); err != nil {
		return err
	}
	if err := system.Dup2(slave.Fd(), 2); err != nil {
		return err
	}
	return nil
}

// openTerminal is a clone of os.OpenFile without the O_CLOEXEC
// used to open the pty slave inside the container namespace
func openTerminal(name string, flag int) (*os.File, error) {
	r, e := syscall.Open(name, flag, 0)
	if e != nil {
		return nil, &os.PathError{"open", name, e}
	}
	return os.NewFile(uintptr(r), name), nil
}

// setupVethNetwork uses the Network config if it is not nil to initialize
// the new veth interface inside the container for use by changing the name to eth0
// setting the MTU and IP address along with the default gateway
func setupVethNetwork(config *libcontainer.Network, tempVethName string) error {
	if config != nil {
		if err := network.InterfaceDown(tempVethName); err != nil {
			return fmt.Errorf("interface down %s %s", tempVethName, err)
		}
		if err := network.ChangeInterfaceName(tempVethName, "eth0"); err != nil {
			return fmt.Errorf("change %s to eth0 %s", tempVethName, err)
		}
		if err := network.SetInterfaceIp("eth0", config.Address); err != nil {
			return fmt.Errorf("set eth0 ip %s", err)
		}
		if err := network.SetMtu("eth0", config.Mtu); err != nil {
			return fmt.Errorf("set eth0 mtu to %d %s", config.Mtu, err)
		}
		if err := network.InterfaceUp("eth0"); err != nil {
			return fmt.Errorf("eth0 up %s", err)
		}
		if err := network.SetMtu("lo", config.Mtu); err != nil {
			return fmt.Errorf("set lo mtu to %d %s", config.Mtu, err)
		}
		if err := network.InterfaceUp("lo"); err != nil {
			return fmt.Errorf("lo up %s", err)
		}
		if config.Gateway != "" {
			if err := network.SetDefaultGateway(config.Gateway); err != nil {
				return fmt.Errorf("set gateway to %s %s", config.Gateway, err)
			}
		}
	}
	return nil
}

// getVethName reads from Stdin the temp veth name
// sent by the parent processes after the veth pair
// has been created and setup
func getVethName(pipe io.ReadCloser) (string, error) {
	defer pipe.Close()

	data, err := ioutil.ReadAll(pipe)
	if err != nil {
		return "", fmt.Errorf("error reading from stdin %s", err)
	}
	return string(data), nil
}
