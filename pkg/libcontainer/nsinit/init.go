// +build linux

package nsinit

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/capabilities"
	"github.com/dotcloud/docker/pkg/libcontainer/network"
	"github.com/dotcloud/docker/pkg/system"
	"github.com/dotcloud/docker/pkg/user"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Init is the init process that first runs inside a new namespace to setup mounts, users, networking,
// and other options required for the new container.
func Init(container *libcontainer.Container, uncleanRootfs, console string, syncPipe *SyncPipe, args []string) error {
	rootfs, err := resolveRootfs(uncleanRootfs)
	if err != nil {
		return err
	}
	log.Printf("initializing namespace at %s", rootfs)

	// We always read this as it is a way to sync with the parent as well
	context, err := syncPipe.ReadFromParent()
	if err != nil {
		syncPipe.Close()
		return err
	}
	syncPipe.Close()
	log.Printf("received context from parent %v", context)

	if console != "" {
		log.Printf("setting up console for %s", console)
		// close pipes so that we can replace it with the pty
		closeStdPipes()
		slave, err := openTerminal(console, syscall.O_RDWR)
		if err != nil {
			return fmt.Errorf("open terminal %s", err)
		}
		if err := dupSlave(slave); err != nil {
			return fmt.Errorf("dup2 slave %s", err)
		}
	}
	if _, err := system.Setsid(); err != nil {
		return fmt.Errorf("setsid %s", err)
	}
	if console != "" {
		if err := system.Setctty(); err != nil {
			return fmt.Errorf("setctty %s", err)
		}
	}
	if err := system.ParentDeathSignal(); err != nil {
		return fmt.Errorf("parent deth signal %s", err)
	}
	if err := setupNewMountNamespace(rootfs, console, container.ReadonlyFs); err != nil {
		return fmt.Errorf("setup mount namespace %s", err)
	}
	if err := setupNetwork(container.Network, context); err != nil {
		return fmt.Errorf("setup networking %s", err)
	}
	if err := system.Sethostname(container.Hostname); err != nil {
		return fmt.Errorf("sethostname %s", err)
	}
	log.Printf("dropping capabilities")
	if err := capabilities.DropCapabilities(container); err != nil {
		return fmt.Errorf("drop capabilities %s", err)
	}
	log.Printf("setting user in namespace")
	if err := setupUser(container); err != nil {
		return fmt.Errorf("setup user %s", err)
	}
	if container.WorkingDir != "" {
		if err := system.Chdir(container.WorkingDir); err != nil {
			return fmt.Errorf("chdir to %s %s", container.WorkingDir, err)
		}
	}
	return execArgs(args, container.Env)
}

func execArgs(args []string, env []string) error {
	name, err := exec.LookPath(args[0])
	if err != nil {
		return err
	}
	log.Printf("execing %s goodbye", name)
	if err := system.Exec(name, args[0:], env); err != nil {
		return fmt.Errorf("exec %s", err)
	}
	panic("unreachable")
}

func closeStdPipes() {
	os.Stdin.Close()
	os.Stdout.Close()
	os.Stderr.Close()
}

// resolveRootfs ensures that the current working directory is
// not a symlink and returns the absolute path to the rootfs
func resolveRootfs(uncleanRootfs string) (string, error) {
	rootfs, err := filepath.Abs(uncleanRootfs)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(rootfs)
}

func setupUser(container *libcontainer.Container) error {
	if container.User != "" {
		uid, gid, suppGids, err := user.GetUserGroupSupplementary(container.User, syscall.Getuid(), syscall.Getgid())
		if err != nil {
			return err
		}
		if err := system.Setgroups(suppGids); err != nil {
			return err
		}
		if err := system.Setgid(gid); err != nil {
			return err
		}
		if err := system.Setuid(uid); err != nil {
			return err
		}
	} else {
		if err := system.Setgroups(nil); err != nil {
			return err
		}
		if err := system.Setresgid(0, 0, 0); err != nil {
			return err
		}
		if err := system.Setresuid(0, 0, 0); err != nil {
			return err
		}
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
func setupNetwork(config *libcontainer.Network, context libcontainer.Context) error {
	if config != nil {
		strategy, err := network.GetStrategy(config.Type)
		if err != nil {
			return err
		}
		return strategy.Initialize(config, context)
	}
	return nil
}
