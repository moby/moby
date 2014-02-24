package namespaces

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/execdriver"
	"github.com/dotcloud/docker/execdriver/lxc"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/nsinit"
	"io"
	"io/ioutil"
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
		var container *libcontainer.Container
		f, err := os.Open("container.json")
		if err != nil {
			return err
		}
		if err := json.NewDecoder(f).Decode(&container); err != nil {
			f.Close()
			return err
		}
		f.Close()

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		syncPipe, err := nsinit.NewSyncPipeFromFd(0, uintptr(args.Pipe))
		if err != nil {
			return err
		}
		if err := nsinit.Init(container, cwd, args.Console, syncPipe, args.Args); err != nil {
			return err
		}
		return nil
	})
}

type driver struct {
	root string
}

type info struct {
	ID     string
	driver *driver
}

func (i *info) IsRunning() bool {
	p := filepath.Join(i.driver.root, "containers", i.ID, "root", ".nspid")
	if _, err := os.Stat(p); err == nil {
		return true
	}
	return false
}

func NewDriver(root string) (*driver, error) {
	return &driver{
		root: root,
	}, nil
}

func (d *driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	var (
		term        nsinit.Terminal
		container   = createContainer(c)
		factory     = &dockerCommandFactory{c}
		stateWriter = &dockerStateWriter{
			callback: startCallback,
			c:        c,
			dsw:      &nsinit.DefaultStateWriter{c.Rootfs},
		}
	)
	if c.Tty {
		term = &dockerTtyTerm{
			pipes: pipes,
		}
	} else {
		term = &dockerStdTerm{
			pipes: pipes,
		}
	}
	c.Terminal = term
	if err := writeContainerFile(container, c.Rootfs); err != nil {
		return -1, err
	}
	args := append([]string{c.Entrypoint}, c.Arguments...)
	return nsinit.Exec(container, factory, stateWriter, term, "", args)
}

func (d *driver) Kill(p *execdriver.Command, sig int) error {
	return syscall.Kill(p.Process.Pid, syscall.Signal(sig))
}

func (d *driver) Restore(c *execdriver.Command) error {
	return ErrNotSupported
}

func (d *driver) Info(id string) execdriver.Info {
	return &info{
		ID:     id,
		driver: d,
	}
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

type dockerCommandFactory struct {
	c *execdriver.Command
}

// createCommand will return an exec.Cmd with the Cloneflags set to the proper namespaces
// defined on the container's configuration and use the current binary as the init with the
// args provided
func (d *dockerCommandFactory) Create(container *libcontainer.Container,
	console, logFile string, syncFd uintptr, args []string) *exec.Cmd {
	c := d.c
	// we need to join the rootfs because nsinit will setup the rootfs and chroot
	initPath := filepath.Join(c.Rootfs, c.InitPath)

	c.Path = initPath
	c.Args = append([]string{
		initPath,
		"-driver", DriverName,
		"-console", console,
		"-pipe", fmt.Sprint(syncFd),
		"-log", logFile,
	}, args...)
	c.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: uintptr(nsinit.GetNamespaceFlags(container.Namespaces)),
	}
	c.Env = container.Env
	c.Dir = c.Rootfs

	return &c.Cmd
}

type dockerStateWriter struct {
	dsw      nsinit.StateWriter
	c        *execdriver.Command
	callback execdriver.StartCallback
}

func (d *dockerStateWriter) WritePid(pid int) error {
	err := d.dsw.WritePid(pid)
	if d.callback != nil {
		d.callback(d.c)
	}
	return err
}

func (d *dockerStateWriter) DeletePid() error {
	return d.dsw.DeletePid()
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
			Type:    "veth",
			Context: libcontainer.Context{
				"prefix": "dock",
				"bridge": c.Network.Bridge,
			},
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

type dockerStdTerm struct {
	lxc.StdConsole
	pipes *execdriver.Pipes
}

func (d *dockerStdTerm) Attach(cmd *exec.Cmd) error {
	return d.AttachPipes(cmd, d.pipes)
}

func (d *dockerStdTerm) SetMaster(master *os.File) {
	// do nothing
}

type dockerTtyTerm struct {
	lxc.TtyConsole
	pipes *execdriver.Pipes
}

func (t *dockerTtyTerm) Attach(cmd *exec.Cmd) error {
	go io.Copy(t.pipes.Stdout, t.MasterPty)
	if t.pipes.Stdin != nil {
		go io.Copy(t.MasterPty, t.pipes.Stdin)
	}
	return nil
}

func (t *dockerTtyTerm) SetMaster(master *os.File) {
	t.MasterPty = master
}
