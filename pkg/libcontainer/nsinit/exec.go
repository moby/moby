// +build linux

package nsinit

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/cgroups/fs"
	"github.com/dotcloud/docker/pkg/cgroups/systemd"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/network"
	"github.com/dotcloud/docker/pkg/system"
)

// Exec performes setup outside of a namespace so that a container can be
// executed.  Exec is a high level function for working with container namespaces.
func (ns *linuxNs) Exec(container *libcontainer.Container, term Terminal, pidRoot string, args []string, startCallback func()) (int, error) {
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

	command := ns.commandFactory.Create(container, console, syncPipe.child, args)
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
	if err := WritePid(pidRoot, command.Process.Pid, started); err != nil {
		command.Process.Kill()
		return -1, err
	}
	defer DeletePid(pidRoot)

	// Do this before syncing with child so that no children
	// can escape the cgroup
	cleaner, err := SetupCgroups(container, command.Process.Pid)
	if err != nil {
		command.Process.Kill()
		return -1, err
	}
	if cleaner != nil {
		defer cleaner.Cleanup()
	}

	if err := InitializeNetworking(container, command.Process.Pid, syncPipe); err != nil {
		command.Process.Kill()
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
	status := command.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	return status, err
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

// WritePid writes the namespaced processes pid to pid and it's start time
// to the path specified
func WritePid(path string, pid int, startTime string) error {
	err := ioutil.WriteFile(filepath.Join(path, "pid"), []byte(fmt.Sprint(pid)), 0655)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(path, "start"), []byte(startTime), 0655)
}

// DeletePid removes the pid and started file from disk when the container's process
// dies and the container is cleanly removed
func DeletePid(path string) error {
	err := os.Remove(filepath.Join(path, "pid"))
	if serr := os.Remove(filepath.Join(path, "start")); err == nil {
		err = serr
	}
	return err
}
