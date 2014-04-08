// +build linux

package nsinit

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/network"
	"github.com/dotcloud/docker/pkg/system"
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
	ns.logger.Printf("created sync pipe parent fd %d child fd %d\n", syncPipe.parent.Fd(), syncPipe.child.Fd())

	if container.Tty {
		ns.logger.Println("creating master and console")
		master, console, err = system.CreateMasterAndConsole()
		if err != nil {
			return -1, err
		}
		term.SetMaster(master)
	}

	command := ns.commandFactory.Create(container, console, syncPipe.child, args)
	ns.logger.Println("attach terminal to command")
	if err := term.Attach(command); err != nil {
		return -1, err
	}
	defer term.Close()

	ns.logger.Println("starting command")
	if err := command.Start(); err != nil {
		return -1, err
	}

	started, err := system.GetProcessStartTime(command.Process.Pid)
	if err != nil {
		return -1, err
	}
	ns.logger.Printf("writting pid %d to file\n", command.Process.Pid)
	if err := ns.stateWriter.WritePid(command.Process.Pid, started); err != nil {
		command.Process.Kill()
		return -1, err
	}
	defer func() {
		ns.logger.Println("removing pid file")
		ns.stateWriter.DeletePid()
	}()

	// Do this before syncing with child so that no children
	// can escape the cgroup
	ns.logger.Println("setting cgroups")
	activeCgroup, err := ns.SetupCgroups(container, command.Process.Pid)
	if err != nil {
		command.Process.Kill()
		return -1, err
	}
	if activeCgroup != nil {
		defer activeCgroup.Cleanup()
	}

	ns.logger.Println("setting up network")
	if err := ns.InitializeNetworking(container, command.Process.Pid, syncPipe); err != nil {
		command.Process.Kill()
		return -1, err
	}

	ns.logger.Println("closing sync pipe with child")
	// Sync with child
	syncPipe.Close()

	if err := command.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}
	status := command.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	ns.logger.Printf("process exited with status %d\n", status)
	return status, err
}

func (ns *linuxNs) SetupCgroups(container *libcontainer.Container, nspid int) (cgroups.ActiveCgroup, error) {
	if container.Cgroups != nil {
		return container.Cgroups.Apply(nspid)
	}
	return nil, nil
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
