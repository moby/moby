package native

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/apparmor"
	"github.com/dotcloud/docker/pkg/libcontainer/nsinit"
	"github.com/dotcloud/docker/pkg/system"
	"github.com/dotcloud/docker/runtime/execdriver"
	"io"
	"io/ioutil"
	"log"
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

func init() {
	execdriver.RegisterInitFunc(DriverName, func(args *execdriver.InitArgs) error {
		var (
			container *libcontainer.Container
			ns        = nsinit.NewNsInit(&nsinit.DefaultCommandFactory{}, &nsinit.DefaultStateWriter{args.Root}, createLogger(""))
		)
		f, err := os.Open(filepath.Join(args.Root, "container.json"))
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
	root             string
	initPath         string
	activeContainers map[string]*exec.Cmd
}

func NewDriver(root, initPath string) (*driver, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	if err := apparmor.InstallDefaultProfile(); err != nil {
		return nil, err
	}
	return &driver{
		root:             root,
		initPath:         initPath,
		activeContainers: make(map[string]*exec.Cmd),
	}, nil
}

func (d *driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	// take the Command and populate the libcontainer.Container from it
	container, err := d.createContainer(c)
	if err != nil {
		return -1, err
	}
	d.activeContainers[c.ID] = &c.Cmd

	var (
		term        nsinit.Terminal
		factory     = &dockerCommandFactory{c: c, driver: d}
		stateWriter = &dockerStateWriter{
			callback: startCallback,
			c:        c,
			dsw:      &nsinit.DefaultStateWriter{filepath.Join(d.root, c.ID)},
		}
		ns   = nsinit.NewNsInit(factory, stateWriter, createLogger(os.Getenv("DEBUG")))
		args = append([]string{c.Entrypoint}, c.Arguments...)
	)
	if err := d.createContainerRoot(c.ID); err != nil {
		return -1, err
	}
	defer d.removeContainerRoot(c.ID)

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
	if err := d.writeContainerFile(container, c.ID); err != nil {
		return -1, err
	}
	return ns.Exec(container, term, args)
}

func (d *driver) Kill(p *execdriver.Command, sig int) error {
	return syscall.Kill(p.Process.Pid, syscall.Signal(sig))
}

func (d *driver) Terminate(p *execdriver.Command) error {
	// lets check the start time for the process
	started, err := d.readStartTime(p)
	if err != nil {
		// if we don't have the data on disk then we can assume the process is gone
		// because this is only removed after we know the process has stopped
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	currentStartTime, err := system.GetProcessStartTime(p.Process.Pid)
	if err != nil {
		return err
	}
	if started == currentStartTime {
		err = syscall.Kill(p.Process.Pid, 9)
	}
	d.removeContainerRoot(p.ID)
	return err

}

func (d *driver) readStartTime(p *execdriver.Command) (string, error) {
	data, err := ioutil.ReadFile(filepath.Join(d.root, p.ID, "start"))
	if err != nil {
		return "", err
	}
	return string(data), nil
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

func (d *driver) writeContainerFile(container *libcontainer.Container, id string) error {
	data, err := json.Marshal(container)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(d.root, id, "container.json"), data, 0655)
}

func (d *driver) createContainerRoot(id string) error {
	return os.MkdirAll(filepath.Join(d.root, id), 0655)
}

func (d *driver) removeContainerRoot(id string) error {
	return os.RemoveAll(filepath.Join(d.root, id))
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
	c      *execdriver.Command
	driver *driver
}

// createCommand will return an exec.Cmd with the Cloneflags set to the proper namespaces
// defined on the container's configuration and use the current binary as the init with the
// args provided
func (d *dockerCommandFactory) Create(container *libcontainer.Container, console string, syncFile *os.File, args []string) *exec.Cmd {
	// we need to join the rootfs because nsinit will setup the rootfs and chroot
	initPath := filepath.Join(d.c.Rootfs, d.c.InitPath)

	d.c.Path = d.driver.initPath
	d.c.Args = append([]string{
		initPath,
		"-driver", DriverName,
		"-console", console,
		"-pipe", "3",
		"-root", filepath.Join(d.driver.root, d.c.ID),
		"--",
	}, args...)

	// set this to nil so that when we set the clone flags anything else is reset
	d.c.SysProcAttr = nil
	system.SetCloneFlags(&d.c.Cmd, uintptr(nsinit.GetNamespaceFlags(container.Namespaces)))
	d.c.ExtraFiles = []*os.File{syncFile}

	d.c.Env = container.Env
	d.c.Dir = d.c.Rootfs

	return &d.c.Cmd
}

type dockerStateWriter struct {
	dsw      nsinit.StateWriter
	c        *execdriver.Command
	callback execdriver.StartCallback
}

func (d *dockerStateWriter) WritePid(pid int, started string) error {
	d.c.ContainerPid = pid
	err := d.dsw.WritePid(pid, started)
	if d.callback != nil {
		d.callback(d.c)
	}
	return err
}

func (d *dockerStateWriter) DeletePid() error {
	return d.dsw.DeletePid()
}

func createLogger(debug string) *log.Logger {
	var w io.Writer
	// if we are in debug mode set the logger to stderr
	if debug != "" {
		w = os.Stderr
	} else {
		w = ioutil.Discard
	}
	return log.New(w, "[libcontainer] ", log.LstdFlags)
}
