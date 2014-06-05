// +build linux

package namespaces

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer/cgroups/fs"
	"github.com/dotcloud/docker/pkg/libcontainer/cgroups/systemd"
	"github.com/dotcloud/docker/pkg/libcontainer/network"
	"github.com/dotcloud/docker/pkg/system"
)

// Exec performes setup outside of a namespace so that a container can be
// executed.  Exec is a high level function for working with container namespaces.
func Exec(container *libcontainer.Container, term Terminal, rootfs, dataPath string, args []string, createCommand CreateCommand, startCallback func()) (int, error) {
	var (
		master  *os.File
		console string
		err     error
	)

	// create a pipe so that we can syncronize with the namespaced process and
	// pass the veth name to the child
	syncPipe, err := NewSyncPipe()
	if err != nil {
		return -1, err
	}

	if container.Tty {
		master, console, err = system.CreateMasterAndConsole()
		if err != nil {
			return -1, err
		}
		term.SetMaster(master)
	}

	command := createCommand(container, console, rootfs, dataPath, os.Args[0], syncPipe.child, args)

	if err := term.Attach(command); err != nil {
		return -1, err
	}
	defer term.Close()

	if err := command.Start(); err != nil {
		return -1, err
	}

	started, err := system.GetProcessStartTime(command.Process.Pid)
	if err != nil {
		return -1, err
	}
	if err := WritePid(dataPath, command.Process.Pid, started); err != nil {
		command.Process.Kill()
		command.Wait()
		return -1, err
	}
	defer DeletePid(dataPath)

	// Do this before syncing with child so that no children
	// can escape the cgroup
	cleaner, err := SetupCgroups(container, command.Process.Pid)
	if err != nil {
		command.Process.Kill()
		command.Wait()
		return -1, err
	}
	if cleaner != nil {
		defer cleaner.Cleanup()
	}

	if err := InitializeNetworking(container, command.Process.Pid, syncPipe); err != nil {
		command.Process.Kill()
		command.Wait()
		return -1, err
	}

	// Sync with child
	syncPipe.Close()

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
// init: the progam executed inside the namespaces
// root: the path to the container json file and information
// pipe: sync pipe to syncronize the parent and child processes
// args: the arguemnts to pass to the container to run as the user's program
func DefaultCreateCommand(container *libcontainer.Container, console, rootfs, dataPath, init string, pipe *os.File, args []string) *exec.Cmd {
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

	command := exec.Command(init, append([]string{"init"}, args...)...)
	// make sure the process is executed inside the context of the rootfs
	command.Dir = rootfs
	command.Env = append(os.Environ(), env...)

	system.SetCloneFlags(command, uintptr(GetNamespaceFlags(container.Namespaces)))
	command.SysProcAttr.Pdeathsig = syscall.SIGKILL
	command.ExtraFiles = []*os.File{pipe}

	return command
}

// SetupCgroups applies the cgroup restrictions to the process running in the contaienr based
// on the container's configuration
func SetupCgroups(container *libcontainer.Container, nspid int) (cgroups.ActiveCgroup, error) {
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
func InitializeNetworking(container *libcontainer.Container, nspid int, pipe *SyncPipe) error {
	context := libcontainer.Context{}
	for _, config := range container.Networks {
		strategy, err := network.GetStrategy(config.Type)
		if err != nil {
			return err
		}
		if err := strategy.Create(config, nspid, context); err != nil {
			return err
		}
	}
	return pipe.SendToChild(context)
}

// GetNamespaceFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare, and setns
func GetNamespaceFlags(namespaces map[string]bool) (flag int) {
	for key, enabled := range namespaces {
		if enabled {
			if ns := libcontainer.GetNamespace(key); ns != nil {
				flag |= ns.Value
			}
		}
	}
	return flag
}
