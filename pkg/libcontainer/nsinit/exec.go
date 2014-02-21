// +build linux

package nsinit

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/network"
	"github.com/dotcloud/docker/pkg/libcontainer/utils"
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
func Exec(container *libcontainer.Container, logFile string, args []string) (int, error) {
	var (
		master  *os.File
		console string
		err     error

		inPipe           io.WriteCloser
		outPipe, errPipe io.ReadCloser
	)

	if container.Tty {
		log.Printf("setting up master and console")
		master, console, err = createMasterAndConsole()
		if err != nil {
			return -1, err
		}
	}

	// create a pipe so that we can syncronize with the namespaced process and
	// pass the veth name to the child
	r, w, err := os.Pipe()
	if err != nil {
		return -1, err
	}
	system.UsetCloseOnExec(r.Fd())

	command := createCommand(container, console, logFile, r.Fd(), args)
	if !container.Tty {
		log.Printf("opening pipes on command")
		if inPipe, err = command.StdinPipe(); err != nil {
			return -1, err
		}
		if outPipe, err = command.StdoutPipe(); err != nil {
			return -1, err
		}
		if errPipe, err = command.StderrPipe(); err != nil {
			return -1, err
		}
	}

	log.Printf("staring init")
	if err := command.Start(); err != nil {
		return -1, err
	}
	log.Printf("writting state file")
	if err := writePidFile(command); err != nil {
		command.Process.Kill()
		return -1, err
	}
	defer deletePidFile()

	// Do this before syncing with child so that no children
	// can escape the cgroup
	if container.Cgroups != nil {
		log.Printf("setting up cgroups")
		if err := container.Cgroups.Apply(command.Process.Pid); err != nil {
			command.Process.Kill()
			return -1, err
		}
	}

	if container.Network != nil {
		log.Printf("creating veth pair")
		vethPair, err := initializeContainerVeth(container.Network.Bridge, container.Network.Mtu, command.Process.Pid)
		if err != nil {
			return -1, err
		}
		log.Printf("sending %s as veth pair name", vethPair)
		sendVethName(w, vethPair)
	}

	// Sync with child
	log.Printf("closing sync pipes")
	w.Close()
	r.Close()

	if container.Tty {
		log.Printf("starting copy for tty")
		go io.Copy(os.Stdout, master)
		go io.Copy(master, os.Stdin)

		state, err := setupWindow(master)
		if err != nil {
			command.Process.Kill()
			return -1, err
		}
		defer term.RestoreTerminal(os.Stdin.Fd(), state)
	} else {
		log.Printf("starting copy for std pipes")
		go func() {
			defer inPipe.Close()
			io.Copy(inPipe, os.Stdin)
		}()
		go io.Copy(os.Stdout, outPipe)
		go io.Copy(os.Stderr, errPipe)
	}

	log.Printf("waiting on process")
	if err := command.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}
	log.Printf("process ended")
	return command.ProcessState.Sys().(syscall.WaitStatus).ExitStatus(), nil
}

// sendVethName writes the veth pair name to the child's stdin then closes the
// pipe so that the child stops waiting for more data
func sendVethName(pipe io.Writer, name string) {
	fmt.Fprint(pipe, name)
}

// initializeContainerVeth will create a veth pair and setup the host's
// side of the pair by setting the specified bridge as the master and bringing
// up the interface.
//
// Then will with set the other side of the veth pair into the container's namespaced
// using the pid and returns the veth's interface name to provide to the container to
// finish setting up the interface inside the namespace
func initializeContainerVeth(bridge string, mtu, nspid int) (string, error) {
	name1, name2, err := createVethPair()
	if err != nil {
		return "", err
	}
	log.Printf("veth pair created %s <> %s", name1, name2)
	if err := network.SetInterfaceMaster(name1, bridge); err != nil {
		return "", err
	}
	if err := network.SetMtu(name1, mtu); err != nil {
		return "", err
	}
	if err := network.InterfaceUp(name1); err != nil {
		return "", err
	}
	log.Printf("setting %s inside %d namespace", name2, nspid)
	if err := network.SetInterfaceInNamespacePid(name2, nspid); err != nil {
		return "", err
	}
	return name2, nil
}

func setupWindow(master *os.File) (*term.State, error) {
	ws, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		return nil, err
	}
	if err := term.SetWinsize(master.Fd(), ws); err != nil {
		return nil, err
	}
	return term.SetRawTerminal(os.Stdin.Fd())
}

// createMasterAndConsole will open /dev/ptmx on the host and retreive the
// pts name for use as the pty slave inside the container
func createMasterAndConsole() (*os.File, string, error) {
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

// createVethPair will automatically generage two random names for
// the veth pair and ensure that they have been created
func createVethPair() (name1 string, name2 string, err error) {
	name1, err = utils.GenerateRandomName("dock", 4)
	if err != nil {
		return
	}
	name2, err = utils.GenerateRandomName("dock", 4)
	if err != nil {
		return
	}
	if err = network.CreateVethPair(name1, name2); err != nil {
		return
	}
	return
}

// writePidFile writes the namespaced processes pid to .nspid in the rootfs for the container
func writePidFile(command *exec.Cmd) error {
	return ioutil.WriteFile(".nspid", []byte(fmt.Sprint(command.Process.Pid)), 0655)
}

func deletePidFile() error {
	return os.Remove(".nspid")
}

// createCommand will return an exec.Cmd with the Cloneflags set to the proper namespaces
// defined on the container's configuration and use the current binary as the init with the
// args provided
func createCommand(container *libcontainer.Container, console, logFile string, pipe uintptr, args []string) *exec.Cmd {
	// get our binary name so we can always reexec ourself
	name := os.Args[0]
	command := exec.Command(name, append([]string{
		"-console", console,
		"-pipe", fmt.Sprint(pipe),
		"-log", logFile,
		"init"}, args...)...)

	command.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: uintptr(getNamespaceFlags(container.Namespaces)),
	}
	return command
}
