// +build linux

package nsinit

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"github.com/dotcloud/docker/pkg/apparmor"
	"github.com/dotcloud/docker/pkg/label"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/console"
	"github.com/dotcloud/docker/pkg/libcontainer/mount"
	"github.com/dotcloud/docker/pkg/libcontainer/network"
	"github.com/dotcloud/docker/pkg/libcontainer/security/capabilities"
	"github.com/dotcloud/docker/pkg/libcontainer/utils"
	"github.com/dotcloud/docker/pkg/system"
	"github.com/dotcloud/docker/pkg/user"
)

// Init is the init process that first runs inside a new namespace to setup mounts, users, networking,
// and other options required for the new container.
func (ns *linuxNs) Init(container *libcontainer.Container, uncleanRootfs, consolePath string, syncPipe *SyncPipe, args []string) error {
	rootfs, err := utils.ResolveRootfs(uncleanRootfs)
	if err != nil {
		return err
	}

	// We always read this as it is a way to sync with the parent as well
	ns.logger.Printf("reading from sync pipe fd %d\n", syncPipe.child.Fd())
	context, err := syncPipe.ReadFromParent()
	if err != nil {
		syncPipe.Close()
		return err
	}
	ns.logger.Println("received context from parent")
	syncPipe.Close()

	if consolePath != "" {
		ns.logger.Printf("setting up %s as console\n", consolePath)
		if err := console.OpenAndDup(consolePath); err != nil {
			return err
		}
	}
	if _, err := system.Setsid(); err != nil {
		return fmt.Errorf("setsid %s", err)
	}
	if consolePath != "" {
		if err := system.Setctty(); err != nil {
			return fmt.Errorf("setctty %s", err)
		}
	}
	if err := setupNetwork(container, context); err != nil {
		return fmt.Errorf("setup networking %s", err)
	}

	label.Init()
	ns.logger.Println("setup mount namespace")
	if err := mount.InitializeMountNamespace(rootfs, consolePath, container); err != nil {
		return fmt.Errorf("setup mount namespace %s", err)
	}
	if err := system.Sethostname(container.Hostname); err != nil {
		return fmt.Errorf("sethostname %s", err)
	}
	if err := finalizeNamespace(container); err != nil {
		return fmt.Errorf("finalize namespace %s", err)
	}

	if profile := container.Context["apparmor_profile"]; profile != "" {
		ns.logger.Printf("setting apparmor profile %s\n", profile)
		if err := apparmor.ApplyProfile(os.Getpid(), profile); err != nil {
			return err
		}
	}
	runtime.LockOSThread()
	if err := label.SetProcessLabel(container.Context["process_label"]); err != nil {
		return fmt.Errorf("SetProcessLabel label %s", err)
	}
	ns.logger.Printf("execing %s\n", args[0])
	return system.Execv(args[0], args[0:], container.Env)
}

func setupUser(container *libcontainer.Container) error {
	uid, gid, suppGids, err := user.GetUserGroupSupplementary(container.User, syscall.Getuid(), syscall.Getgid())
	if err != nil {
		return fmt.Errorf("GetUserGroupSupplementary %s", err)
	}
	if err := system.Setgroups(suppGids); err != nil {
		return fmt.Errorf("setgroups %s", err)
	}
	if err := system.Setgid(gid); err != nil {
		return fmt.Errorf("setgid %s", err)
	}
	if err := system.Setuid(uid); err != nil {
		return fmt.Errorf("setuid %s", err)
	}
	return nil
}

// setupVethNetwork uses the Network config if it is not nil to initialize
// the new veth interface inside the container for use by changing the name to eth0
// setting the MTU and IP address along with the default gateway
func setupNetwork(container *libcontainer.Container, context libcontainer.Context) error {
	for _, config := range container.Networks {
		strategy, err := network.GetStrategy(config.Type)
		if err != nil {
			return err
		}

		err1 := strategy.Initialize(config, context)
		if err1 != nil {
			return err1
		}
	}
	return nil
}

// finalizeNamespace drops the caps, sets the correct user
// and working dir, and closes any leaky file descriptors
// before execing the command inside the namespace
func finalizeNamespace(container *libcontainer.Container) error {
	if err := capabilities.DropCapabilities(container); err != nil {
		return fmt.Errorf("drop capabilities %s", err)
	}
	if err := system.CloseFdsFrom(3); err != nil {
		return fmt.Errorf("close open file descriptors %s", err)
	}
	if err := setupUser(container); err != nil {
		return fmt.Errorf("setup user %s", err)
	}
	if container.WorkingDir != "" {
		if err := system.Chdir(container.WorkingDir); err != nil {
			return fmt.Errorf("chdir to %s %s", container.WorkingDir, err)
		}
	}
	return nil
}
