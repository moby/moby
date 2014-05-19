// +build linux

package nsinit

/*
#include <linux/securebits.h>
*/
import "C"

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/dotcloud/docker/pkg/apparmor"
	"github.com/dotcloud/docker/pkg/label"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/console"
	"github.com/dotcloud/docker/pkg/libcontainer/mount"
	"github.com/dotcloud/docker/pkg/libcontainer/network"
	"github.com/dotcloud/docker/pkg/libcontainer/security/capabilities"
	"github.com/dotcloud/docker/pkg/libcontainer/security/restrict"
	"github.com/dotcloud/docker/pkg/libcontainer/utils"
	"github.com/dotcloud/docker/pkg/netlink"
	"github.com/dotcloud/docker/pkg/system"
	"github.com/dotcloud/docker/pkg/user"
)

// Init is the init process that first runs inside a new namespace to setup mounts, users, networking,
// and other options required for the new container.
func Init(container *libcontainer.Container, uncleanRootfs, consolePath string, syncPipe *SyncPipe, args []string) error {
	rootfs, err := utils.ResolveRootfs(uncleanRootfs)
	if err != nil {
		return err
	}

	// clear the current processes env and replace it with the environment
	// defined on the container
	if err := LoadContainerEnvironment(container); err != nil {
		return err
	}

	// We always read this as it is a way to sync with the parent as well
	context, err := syncPipe.ReadFromParent()
	if err != nil {
		syncPipe.Close()
		return err
	}
	syncPipe.Close()

	if consolePath != "" {
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
	if err := setupRoute(container); err != nil {
		return fmt.Errorf("setup route %s", err)
	}

	label.Init()

	if err := mount.InitializeMountNamespace(rootfs, consolePath, container); err != nil {
		return fmt.Errorf("setup mount namespace %s", err)
	}
	if container.Hostname != "" {
		if err := system.Sethostname(container.Hostname); err != nil {
			return fmt.Errorf("sethostname %s", err)
		}
	}

	runtime.LockOSThread()

	if err := apparmor.ApplyProfile(container.Context["apparmor_profile"]); err != nil {
		return fmt.Errorf("set apparmor profile %s: %s", container.Context["apparmor_profile"], err)
	}
	if err := label.SetProcessLabel(container.Context["process_label"]); err != nil {
		return fmt.Errorf("set process label %s", err)
	}
	if container.Context["restrictions"] != "" {
		if err := restrict.Restrict("proc/sys", "proc/sysrq-trigger", "proc/irq", "proc/bus", "sys"); err != nil {
			return err
		}
	}

	pdeathSignal, err := system.GetParentDeathSignal()
	if err != nil {
		return fmt.Errorf("get parent death signal %s", err)
	}

	if !container.MapDockerRoot {
		if err := FinalizeNamespace(container); err != nil {
			return fmt.Errorf("finalize namespace %s", err)
		}
	}

	// Retain capabilities on clone.
	if container.MapDockerRoot {
		if err := system.Prctl(syscall.PR_SET_SECUREBITS, uintptr(C.SECBIT_KEEP_CAPS|C.SECBIT_NO_SETUID_FIXUP), 0, 0, 0); err != nil {
			return fmt.Errorf("prctl %s", err)
		}

		// Switch to the docker-root user.
		if err := system.Setuid(container.DockerRootUid); err != nil {
			return fmt.Errorf("setuid %s", err)
		}

		// Switch to the docker-root group.
		if err := system.Setgid(container.DockerRootGid); err != nil {
			return fmt.Errorf("setgid %s", err)
		}
	}

	// Changing user/group clears the parent death
	// signal, so we restore it here.
	if err := RestoreParentDeathSignal(pdeathSignal); err != nil {
		return fmt.Errorf("restore parent death signal %s", err)
	}

	// Non user namespace case.
	if !container.MapDockerRoot {
		return system.Execv(args[0], args[0:], container.Env)
	}

	// Switch into a user namespace and map docker-root to root.
	sPipe, err := NewSyncPipe()
	if err != nil {
		return err
	}

	// Prepare arguments for the raw syscalls.
	var (
		r1   uintptr
		err1 syscall.Errno
		dir  *byte
	)

	argv0p, err := syscall.BytePtrFromString(args[0])
	if err != nil {
		return err
	}

	argvp, err := syscall.SlicePtrFromStrings(args[0:])
	if err != nil {
		return err
	}

	envvp, err := syscall.SlicePtrFromStrings(container.Env)
	if err != nil {
		return err
	}

	if container.WorkingDir != "" {
		dir, err = syscall.BytePtrFromString(container.WorkingDir)
		if err != nil {
			return err
		}
	}

	syscall.ForkLock.Lock()
	r1, _, err1 = syscall.RawSyscall6(syscall.SYS_CLONE, uintptr(syscall.CLONE_NEWUSER|syscall.CLONE_FILES|syscall.SIGCHLD), 0, 0, 0, 0, 0)
	if err1 != 0 {
		return fmt.Errorf("userns clone: %s", err)
	}

	if r1 != 0 {
		// In parent.
		syscall.ForkLock.Unlock()
		proc, err := os.FindProcess(int(r1))
		if err != nil {
			return err
		}

		if err = writeUserMappings(int(r1), container.UidMappings, container.GidMappings); err != nil {
			proc.Kill()
			return fmt.Errorf("Failed to write mappings: %s", err)
		}
		sPipe.Close()

		state, err := proc.Wait()
		if err != nil {
			proc.Kill()
			return fmt.Errorf("wait: %s", err)
		}
		os.Exit(state.Sys().(syscall.WaitStatus).ExitStatus())
	}

	// In child.
	if dir != nil {
		_, _, err1 = syscall.RawSyscall(syscall.SYS_CHDIR, uintptr(unsafe.Pointer(dir)), 0, 0)
		if err1 != 0 {
			return err1
		}
	}

	_, _, err1 = syscall.RawSyscall(syscall.SYS_EXECVE,
		uintptr(unsafe.Pointer(argv0p)),
		uintptr(unsafe.Pointer(&argvp[0])),
		uintptr(unsafe.Pointer(&envvp[0])))

	return nil
}

// Write UID/GID mappings for a process.
func writeUserMappings(pid int, uidMappings, gidMappings []string) error {
	if len(uidMappings) > 5 || len(gidMappings) > 5 {
		return fmt.Errorf("Only 5 uid/gid mappings are supported by the kernel")
	}

	uidMap := []byte(strings.Join(uidMappings, "\n"))
	gidMap := []byte(strings.Join(gidMappings, "\n"))

	uidMappingsFile := fmt.Sprintf("/proc/%v/uid_map", pid)
	gidMappingsFile := fmt.Sprintf("/proc/%v/gid_map", pid)

	if err := ioutil.WriteFile(uidMappingsFile, uidMap, 0644); err != nil {
		return err
	}
	if err := ioutil.WriteFile(gidMappingsFile, gidMap, 0644); err != nil {
		return err
	}
	return nil
}

// RestoreParentDeathSignal sets the parent death signal to old.
func RestoreParentDeathSignal(old int) error {
	if old == 0 {
		return nil
	}

	current, err := system.GetParentDeathSignal()
	if err != nil {
		return fmt.Errorf("get parent death signal %s", err)
	}

	if old == current {
		return nil
	}

	if err := system.ParentDeathSignal(uintptr(old)); err != nil {
		return fmt.Errorf("set parent death signal %s", err)
	}

	// Signal self if parent is already dead. Does nothing if running in a new
	// PID namespace, as Getppid will always return 0.
	if syscall.Getppid() == 1 {
		return syscall.Kill(syscall.Getpid(), syscall.SIGKILL)
	}

	return nil
}

// SetupUser changes the groups, gid, and uid for the user inside the container
func SetupUser(u string) error {
	uid, gid, suppGids, err := user.GetUserGroupSupplementary(u, syscall.Getuid(), syscall.Getgid())
	if err != nil {
		return fmt.Errorf("get supplementary groups %s", err)
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

func setupRoute(container *libcontainer.Container) error {
	for _, config := range container.Routes {
		if err := netlink.AddRoute(config.Destination, config.Source, config.Gateway, config.InterfaceName); err != nil {
			return err
		}
	}
	return nil
}

// FinalizeNamespace drops the caps, sets the correct user
// and working dir, and closes any leaky file descriptors
// before execing the command inside the namespace
func FinalizeNamespace(container *libcontainer.Container) error {
	if err := system.CloseFdsFrom(3); err != nil {
		return fmt.Errorf("close open file descriptors %s", err)
	}

	// drop capabilities in bounding set before changing user
	if err := capabilities.DropBoundingSet(container); err != nil {
		return fmt.Errorf("drop bounding set %s", err)
	}

	// preserve existing capabilities while we change users
	if err := system.SetKeepCaps(); err != nil {
		return fmt.Errorf("set keep caps %s", err)
	}

	if err := SetupUser(container.User); err != nil {
		return fmt.Errorf("setup user %s", err)
	}

	if err := system.ClearKeepCaps(); err != nil {
		return fmt.Errorf("clear keep caps %s", err)
	}

	// drop all other capabilities
	if err := capabilities.DropCapabilities(container); err != nil {
		return fmt.Errorf("drop capabilities %s", err)
	}

	if container.WorkingDir != "" {
		if err := system.Chdir(container.WorkingDir); err != nil {
			return fmt.Errorf("chdir to %s %s", container.WorkingDir, err)
		}
	}
	return nil
}

func LoadContainerEnvironment(container *libcontainer.Container) error {
	os.Clearenv()
	for _, pair := range container.Env {
		p := strings.SplitN(pair, "=", 2)
		if len(p) < 2 {
			return fmt.Errorf("invalid environment '%v'", pair)
		}
		if err := os.Setenv(p[0], p[1]); err != nil {
			return err
		}
	}
	return nil
}
