// +build linux

package nsinit

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/network"
	"github.com/dotcloud/docker/pkg/system"
	"log"
	"os"
	"os/exec"
	"syscall"
)

// Exec performes setup outside of a namespace so that a container can be
// executed.  Exec is a high level function for working with container namespaces.
func Exec(container *libcontainer.Container,
	factory CommandFactory, state StateWriter, term Terminal,
	logFile string, args []string) (int, error) {
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
		log.Printf("setting up master and console")
		master, console, err = CreateMasterAndConsole()
		if err != nil {
			return -1, err
		}
		term.SetMaster(master)
	}

	command := factory.Create(container, console, logFile, syncPipe.child.Fd(), args)
	if err := term.Attach(command); err != nil {
		return -1, err
	}
	defer term.Close()

	log.Printf("staring init")
	if err := command.Start(); err != nil {
		return -1, err
	}
	log.Printf("writing state file")
	if err := state.WritePid(command.Process.Pid); err != nil {
		command.Process.Kill()
		return -1, err
	}
	defer func() {
		log.Printf("removing state file")
		state.DeletePid()
	}()

	// Do this before syncing with child so that no children
	// can escape the cgroup
	if err := SetupCgroups(container, command.Process.Pid); err != nil {
		command.Process.Kill()
		return -1, err
	}
	if err := InitializeNetworking(container, command.Process.Pid, syncPipe); err != nil {
		command.Process.Kill()
		return -1, err
	}

	// Sync with child
	log.Printf("closing sync pipes")
	syncPipe.Close()

	log.Printf("waiting on process")
	if err := command.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}
	log.Printf("process ended")
	return command.ProcessState.Sys().(syscall.WaitStatus).ExitStatus(), nil
}

func SetupCgroups(container *libcontainer.Container, nspid int) error {
	if container.Cgroups != nil {
		log.Printf("setting up cgroups")
		if err := container.Cgroups.Apply(nspid); err != nil {
			return err
		}
	}
	return nil
}

func InitializeNetworking(container *libcontainer.Container, nspid int, pipe *SyncPipe) error {
	if container.Network != nil {
		log.Printf("creating host network configuration type %s", container.Network.Type)
		strategy, err := network.GetStrategy(container.Network.Type)
		if err != nil {
			return err
		}
		networkContext, err := strategy.Create(container.Network, nspid)
		if err != nil {
			return err
		}
		log.Printf("sending %v as network context", networkContext)
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
