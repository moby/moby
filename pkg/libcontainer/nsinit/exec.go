// +build linux

package nsinit

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/network"
	"github.com/dotcloud/docker/pkg/system"
	"github.com/dotcloud/docker/pkg/term"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"syscall"
)

// Exec performes setup outside of a namespace so that a container can be
// executed.  Exec is a high level function for working with container namespaces.
func Exec(container *libcontainer.Container, stdin io.Reader, stdout, stderr io.Writer,
	master *os.File, logFile string, args []string) (int, error) {
	var (
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
	}

	command := CreateCommand(container, console, logFile, syncPipe.child.Fd(), args)
	if container.Tty {
		log.Printf("starting copy for tty")
		go io.Copy(stdout, master)
		go io.Copy(master, stdin)

		state, err := SetupWindow(master, os.Stdin)
		if err != nil {
			command.Process.Kill()
			return -1, err
		}
		defer term.RestoreTerminal(os.Stdin.Fd(), state)
	} else {
		if err := startStdCopy(command, stdin, stdout, stderr); err != nil {
			command.Process.Kill()
			return -1, err
		}
	}

	log.Printf("staring init")
	if err := command.Start(); err != nil {
		return -1, err
	}
	log.Printf("writing state file")
	if err := writePidFile(command); err != nil {
		command.Process.Kill()
		return -1, err
	}
	defer deletePidFile()

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

// SetupWindow gets the parent window size and sets the master
// pty to the current size and set the parents mode to RAW
func SetupWindow(master, parent *os.File) (*term.State, error) {
	ws, err := term.GetWinsize(parent.Fd())
	if err != nil {
		return nil, err
	}
	if err := term.SetWinsize(master.Fd(), ws); err != nil {
		return nil, err
	}
	return term.SetRawTerminal(parent.Fd())
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

// writePidFile writes the namespaced processes pid to .nspid in the rootfs for the container
func writePidFile(command *exec.Cmd) error {
	return ioutil.WriteFile(".nspid", []byte(fmt.Sprint(command.Process.Pid)), 0655)
}

func deletePidFile() error {
	log.Printf("removing .nspid file")
	return os.Remove(".nspid")
}

// createCommand will return an exec.Cmd with the Cloneflags set to the proper namespaces
// defined on the container's configuration and use the current binary as the init with the
// args provided
func CreateCommand(container *libcontainer.Container, console, logFile string, pipe uintptr, args []string) *exec.Cmd {
	// get our binary name so we can always reexec ourself
	name := os.Args[0]
	command := exec.Command(name, append([]string{
		"-console", console,
		"-pipe", fmt.Sprint(pipe),
		"-log", logFile,
		"init"}, args...)...)

	command.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: uintptr(GetNamespaceFlags(container.Namespaces)),
	}
	command.Env = container.Env
	return command
}

func startStdCopy(command *exec.Cmd, stdin io.Reader, stdout, stderr io.Writer) error {
	log.Printf("opening std pipes")
	inPipe, err := command.StdinPipe()
	if err != nil {
		return err
	}
	outPipe, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	errPipe, err := command.StderrPipe()
	if err != nil {
		return err
	}

	log.Printf("starting copy for std pipes")
	go func() {
		defer inPipe.Close()
		io.Copy(inPipe, stdin)
	}()
	go io.Copy(stdout, outPipe)
	go io.Copy(stderr, errPipe)

	return nil
}
