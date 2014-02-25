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
		ns.logger.Printf("setting up master and console")
		master, console, err = CreateMasterAndConsole()
		if err != nil {
			return -1, err
		}
		term.SetMaster(master)
	}

	command := ns.commandFactory.Create(container, console, ns.logFile, syncPipe.child.Fd(), args)
	if err := term.Attach(command); err != nil {
		return -1, err
	}
	defer term.Close()

	ns.logger.Printf("staring init")
	if err := command.Start(); err != nil {
		return -1, err
	}
	ns.logger.Printf("writing state file")
	if err := ns.stateWriter.WritePid(command.Process.Pid); err != nil {
		command.Process.Kill()
		return -1, err
	}
	defer func() {
		ns.logger.Printf("removing state file")
		ns.stateWriter.DeletePid()
	}()

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
	ns.logger.Printf("closing sync pipes")
	syncPipe.Close()

	ns.logger.Printf("waiting on process")
	if err := command.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}

	exitCode := command.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	ns.logger.Printf("process ended with exit code %d", exitCode)
	return exitCode, nil
}

func (ns *linuxNs) SetupCgroups(container *libcontainer.Container, nspid int) error {
	if container.Cgroups != nil {
		ns.logger.Printf("setting up cgroups")
		if err := container.Cgroups.Apply(nspid); err != nil {
			return err
		}
	}
	return nil
}

func (ns *linuxNs) InitializeNetworking(container *libcontainer.Container, nspid int, pipe *SyncPipe) error {
	if container.Network != nil {
		ns.logger.Printf("creating host network configuration type %s", container.Network.Type)
		strategy, err := network.GetStrategy(container.Network.Type)
		if err != nil {
			return err
		}
		networkContext, err := strategy.Create(container.Network, nspid)
		if err != nil {
			return err
		}
		ns.logger.Printf("sending %v as network context", networkContext)
		if err := pipe.SendToChild(networkContext); err != nil {
			return err
		}
	}
	return nil
}

// CreateMasterAndConsole will open /dev/ptmx on the host and retreive the
// pts name for use as the pty slave inside the container
func CreateMasterAndConsole() (*os.File, string, error) {
	master, err := os.OpenFile("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", err
	}
	console, err := system.Ptsname(master)
	if err != nil {
		return nil, "", err
	}
	if err := system.Unlockpt(master); err != nil {
		return nil, "", err
	}
	return master, console, nil
}
