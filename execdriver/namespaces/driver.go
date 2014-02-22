package namespaces

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/execdriver"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/network"
	"github.com/dotcloud/docker/pkg/libcontainer/nsinit"
	"github.com/dotcloud/docker/pkg/libcontainer/utils"
	"github.com/dotcloud/docker/pkg/system"
	"github.com/dotcloud/docker/pkg/term"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	DriverName = "namespaces"
	Version    = "0.1"
)

var (
	ErrNotSupported = errors.New("not supported")
)

func init() {
	execdriver.RegisterInitFunc(DriverName, func(args *execdriver.InitArgs) error {
		return nil
	})
}

type driver struct {
}

func NewDriver() (*driver, error) {
	return &driver{}, nil
}

func (d *driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	container := createContainer(c)
	if err := writeContainerFile(container, c.Rootfs); err != nil {
		return -1, err
	}

	var (
		console string
		master  *os.File
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
	c.Terminal = NewTerm(pipes, master)

	// create a pipe so that we can syncronize with the namespaced process and
	// pass the veth name to the child
	r, w, err := os.Pipe()
	if err != nil {
		return -1, err
	}
	system.UsetCloseOnExec(r.Fd())

	args := append([]string{c.Entrypoint}, c.Arguments...)
	createCommand(c, container, console, "/nsinit.logs", r.Fd(), args)
	command := c

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
	if err := writePidFile(c.Rootfs, command.Process.Pid); err != nil {
		command.Process.Kill()
		return -1, err
	}
	defer deletePidFile(c.Rootfs)

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
		go io.Copy(pipes.Stdout, master)
		if pipes.Stdin != nil {
			go io.Copy(master, pipes.Stdin)
		}

		/*
			state, err := setupWindow(master)
			if err != nil {
				command.Process.Kill()
				return -1, err
			}
			defer term.RestoreTerminal(uintptr(syscall.Stdin), state)
		*/
	} else {
		log.Printf("starting copy for std pipes")
		if pipes.Stdin != nil {
			go func() {
				defer inPipe.Close()
				io.Copy(inPipe, pipes.Stdin)
			}()
		}
		go io.Copy(pipes.Stdout, outPipe)
		go io.Copy(pipes.Stderr, errPipe)
	}

	if startCallback != nil {
		startCallback(c)
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

func (d *driver) Kill(p *execdriver.Command, sig int) error {
	return p.Process.Kill()
}

func (d *driver) Restore(c *execdriver.Command) error {
	return ErrNotSupported
}

func (d *driver) Info(id string) execdriver.Info {
	return nil
}

func (d *driver) Name() string {
	return fmt.Sprintf("%s-%s", DriverName, Version)
}

func (d *driver) GetPidsForContainer(id string) ([]int, error) {
	return nil, ErrNotSupported
}

func writeContainerFile(container *libcontainer.Container, rootfs string) error {
	data, err := json.Marshal(container)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(rootfs, "container.json"), data, 0755)
}

func getEnv(key string, env []string) string {
	for _, pair := range env {
		parts := strings.Split(pair, "=")
		if parts[0] == key {
			return parts[1]
		}
	}
	return ""
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
func writePidFile(rootfs string, pid int) error {
	return ioutil.WriteFile(filepath.Join(rootfs, ".nspid"), []byte(fmt.Sprint(pid)), 0655)
}

func deletePidFile(rootfs string) error {
	return os.Remove(filepath.Join(rootfs, ".nspid"))
}

// createCommand will return an exec.Cmd with the Cloneflags set to the proper namespaces
// defined on the container's configuration and use the current binary as the init with the
// args provided
func createCommand(c *execdriver.Command, container *libcontainer.Container,
	console, logFile string, pipe uintptr, args []string) {

	aname, _ := exec.LookPath("nsinit")
	c.Path = aname
	c.Args = append([]string{
		aname,
		"-console", console,
		"-pipe", fmt.Sprint(pipe),
		"-log", logFile,
		"init",
	}, args...)
	c.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: uintptr(nsinit.GetNamespaceFlags(container.Namespaces)),
	}
	c.Env = container.Env
	c.Dir = c.Rootfs
}

func createContainer(c *execdriver.Command) *libcontainer.Container {
	container := getDefaultTemplate()

	container.Hostname = getEnv("HOSTNAME", c.Env)
	container.Tty = c.Tty
	container.User = c.User
	container.WorkingDir = c.WorkingDir
	container.Env = c.Env

	container.Env = append(container.Env, "container=docker")

	if c.Network != nil {
		container.Network = &libcontainer.Network{
			Mtu:     c.Network.Mtu,
			Address: fmt.Sprintf("%s/%d", c.Network.IPAddress, c.Network.IPPrefixLen),
			Gateway: c.Network.Gateway,
			Bridge:  c.Network.Bridge,
		}
	}
	if c.Privileged {
		container.Capabilities = nil
	}
	if c.Resources != nil {
		container.Cgroups.CpuShares = c.Resources.CpuShares
		container.Cgroups.Memory = c.Resources.Memory
		container.Cgroups.MemorySwap = c.Resources.MemorySwap
	}
	return container
}
