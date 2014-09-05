// +build linux

package namespaces

import (
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/cgroups/systemd"
	"github.com/docker/libcontainer/network"
	"github.com/docker/libcontainer/syncpipe"
	"github.com/docker/libcontainer/system"
)

// TODO(vishh): This is part of the libcontainer API and it does much more than just namespaces related work.
// Move this to libcontainer package.
// Exec performs setup outside of a namespace so that a container can be
// executed.  Exec is a high level function for working with container namespaces.
func Exec(container *libcontainer.Config, stdin io.Reader, stdout, stderr io.Writer, console string, rootfs, dataPath string, args []string, createCommand CreateCommand, startCallback func()) (int, error) {
	var (
		err error
	)

	// create a pipe so that we can syncronize with the namespaced process and
	// pass the veth name to the child
	syncPipe, err := syncpipe.NewSyncPipe()
	if err != nil {
		return -1, err
	}
	defer syncPipe.Close()

	command := createCommand(container, console, rootfs, dataPath, os.Args[0], syncPipe.Child(), args)
	// Note: these are only used in non-tty mode
	// if there is a tty for the container it will be opened within the namespace and the
	// fds will be duped to stdin, stdiout, and stderr
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr

	if err := command.Start(); err != nil {
		return -1, err
	}

	// Now we passed the pipe to the child, close our side
	syncPipe.CloseChild()

	started, err := system.GetProcessStartTime(command.Process.Pid)
	if err != nil {
		return -1, err
	}

	// Do this before syncing with child so that no children
	// can escape the cgroup
	cgroupRef, err := SetupCgroups(container, command.Process.Pid)
	if err != nil {
		command.Process.Kill()
		command.Wait()
		return -1, err
	}
	defer cgroupRef.Cleanup()

	cgroupPaths, err := cgroupRef.Paths()
	if err != nil {
		command.Process.Kill()
		command.Wait()
		return -1, err
	}

	var networkState network.NetworkState
	if err := InitializeNetworking(container, command.Process.Pid, syncPipe, &networkState); err != nil {
		command.Process.Kill()
		command.Wait()
		return -1, err
	}

	state := &libcontainer.State{
		InitPid:       command.Process.Pid,
		InitStartTime: started,
		NetworkState:  networkState,
		CgroupPaths:   cgroupPaths,
	}

	if err := libcontainer.SaveState(dataPath, state); err != nil {
		command.Process.Kill()
		command.Wait()
		return -1, err
	}
	defer libcontainer.DeleteState(dataPath)

	// Sync with child
	if err := syncPipe.ReadFromChild(); err != nil {
		command.Process.Kill()
		command.Wait()
		return -1, err
	}

	if startCallback != nil {
		startCallback()
	}

	if err := command.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}

	return command.ProcessState.Sys().(syscall.WaitStatus).ExitStatus(), nil
}

// DefaultCreateCommand will return an exec.Cmd with the Cloneflags set to the proper namespaces
// defined on the container's configuration and use the current binary as the init with the
// args provided
//
// console: the /dev/console to setup inside the container
// init: the program executed inside the namespaces
// root: the path to the container json file and information
// pipe: sync pipe to synchronize the parent and child processes
// args: the arguments to pass to the container to run as the user's program
func DefaultCreateCommand(container *libcontainer.Config, console, rootfs, dataPath, init string, pipe *os.File, args []string) *exec.Cmd {
	// get our binary name from arg0 so we can always reexec ourself
	env := []string{
		"console=" + console,
		"pipe=3",
		"data_path=" + dataPath,
	}

	/*
	   TODO: move user and wd into env
	   if user != "" {
	       env = append(env, "user="+user)
	   }
	   if workingDir != "" {
	       env = append(env, "wd="+workingDir)
	   }
	*/

	command := exec.Command(init, append([]string{"init", "--"}, args...)...)
	// make sure the process is executed inside the context of the rootfs
	command.Dir = rootfs
	command.Env = append(os.Environ(), env...)

	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Cloneflags = uintptr(GetNamespaceFlags(container.Namespaces))

	command.SysProcAttr.Pdeathsig = syscall.SIGKILL
	command.ExtraFiles = []*os.File{pipe}

	return command
}

// SetupCgroups applies the cgroup restrictions to the process running in the container based
// on the container's configuration
func SetupCgroups(container *libcontainer.Config, nspid int) (cgroups.ActiveCgroup, error) {
	if container.Cgroups != nil {
		c := container.Cgroups

		if systemd.UseSystemd() {
			return systemd.Apply(c, nspid)
		}

		return fs.Apply(c, nspid)
	}

	return nil, nil
}

// InitializeNetworking creates the container's network stack outside of the namespace and moves
// interfaces into the container's net namespaces if necessary
func InitializeNetworking(container *libcontainer.Config, nspid int, pipe *syncpipe.SyncPipe, networkState *network.NetworkState) error {
	for _, config := range container.Networks {
		strategy, err := network.GetStrategy(config.Type)
		if err != nil {
			return err
		}
		if err := strategy.Create((*network.Network)(config), nspid, networkState); err != nil {
			return err
		}
	}
	return pipe.SendToChild(networkState)
}

// GetNamespaceFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare, and setns
func GetNamespaceFlags(namespaces map[string]bool) (flag int) {
	for key, enabled := range namespaces {
		if enabled {
			if ns := GetNamespace(key); ns != nil {
				flag |= ns.Value
			}
		}
	}
	return flag
}
