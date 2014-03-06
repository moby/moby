// +build linux

package nsinit

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/network"
	"github.com/dotcloud/docker/pkg/system"
	"os"
	"os/exec"
	"syscall"
)

// Exec performes setup outside of a namespace so that a container can be
// executed.  Exec is a high level function for working with container namespaces.
func (ns *linuxNs) Exec(container *libcontainer.Container, term Terminal, args []string) (int, error) {
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
	if err := ns.stateWriter.WritePid(command.Process.Pid); err != nil {
		command.Process.Kill()
		return -1, err
	}
	defer ns.stateWriter.DeletePid()

	// Do this before syncing with child so that no children
	// can escape the cgroup
	if err := ns.SetupCgroups(container, command.Process.Pid); err != nil {
		command.Process.Kill()
		return -1, err
	}
	if err := ns.InitializeNetworking(container, command.Process.Pid, syncPipe); err != nil {
		command.Process.Kill()
		return -1, err
	}

	// Sync with child
	syncPipe.Close()

	if err := command.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}
	return command.ProcessState.Sys().(syscall.WaitStatus).ExitStatus(), nil
}

func (ns *linuxNs) SetupCgroups(container *libcontainer.Container, nspid int) error {
	if container.Cgroups != nil {
		if err := container.Cgroups.Apply(nspid); err != nil {
			return err
		}
	}
	return nil
}

func (ns *linuxNs) InitializeNetworking(container *libcontainer.Container, nspid int, pipe *SyncPipe) error {
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
