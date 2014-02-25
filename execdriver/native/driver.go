package native

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/execdriver"
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/nsinit"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	DriverName = "native"
	Version    = "0.1"
)

var (
	ErrNotSupported = errors.New("not supported")
)

func init() {
	execdriver.RegisterInitFunc(DriverName, func(args *execdriver.InitArgs) error {
		var (
			container *libcontainer.Container
			ns        = nsinit.NewNsInit(&nsinit.DefaultCommandFactory{}, &nsinit.DefaultStateWriter{})
		)
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
		if err := ns.Init(container, cwd, args.Console, syncPipe, args.Args); err != nil {
			return err
		}
		return nil
	})
}

type driver struct {
	root string
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
		ns = nsinit.NewNsInit(factory, stateWriter)
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
	return ns.Exec(container, term, args)
}

func (d *driver) Kill(p *execdriver.Command, sig int) error {
	return syscall.Kill(p.Process.Pid, syscall.Signal(sig))
}

func (d *driver) Restore(c *execdriver.Command) error {
	var (
		nspid int
		p     = filepath.Join(d.root, "containers", c.ID, "root", ".nspid")
	)
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fscanf(f, "%d", &nspid); err != nil {
		return err
	}
	proc, err := os.FindProcess(nspid)
	if err != nil {
		return err
	}
	_, err = proc.Wait()
	return err
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

// TODO: this can be improved with our driver
// there has to be a better way to do this
func (d *driver) GetPidsForContainer(id string) ([]int, error) {
	pids := []int{}

	subsystem := "devices"
	cgroupRoot, err := cgroups.FindCgroupMountpoint(subsystem)
	if err != nil {
		return pids, err
	}
	cgroupDir, err := cgroups.GetThisCgroupDir(subsystem)
	if err != nil {
		return pids, err
	}

	filename := filepath.Join(cgroupRoot, cgroupDir, id, "tasks")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		filename = filepath.Join(cgroupRoot, cgroupDir, "docker", id, "tasks")
	}

	output, err := ioutil.ReadFile(filename)
	if err != nil {
		return pids, err
	}
	for _, p := range strings.Split(string(output), "\n") {
		if len(p) == 0 {
			continue
		}
		pid, err := strconv.Atoi(p)
		if err != nil {
			return pids, fmt.Errorf("Invalid pid '%s': %s", p, err)
		}
		pids = append(pids, pid)
	}
	return pids, nil
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
func (d *dockerCommandFactory) Create(container *libcontainer.Container, console string, syncFd uintptr, args []string) *exec.Cmd {
	// we need to join the rootfs because nsinit will setup the rootfs and chroot
	initPath := filepath.Join(d.c.Rootfs, d.c.InitPath)

	d.c.Path = initPath
	d.c.Args = append([]string{
		initPath,
		"-driver", DriverName,
		"-console", console,
		"-pipe", fmt.Sprint(syncFd),
	}, args...)
	d.c.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: uintptr(nsinit.GetNamespaceFlags(container.Namespaces)),
	}
	d.c.Env = container.Env
	d.c.Dir = d.c.Rootfs

	return &d.c.Cmd
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
