package native

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/dotcloud/docker/daemon/execdriver"
	"github.com/dotcloud/docker/pkg/apparmor"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/cgroups/fs"
	"github.com/dotcloud/docker/pkg/libcontainer/cgroups/systemd"
	"github.com/dotcloud/docker/pkg/libcontainer/namespaces"
	"github.com/dotcloud/docker/pkg/system"
)

const (
	DriverName = "native"
	Version    = "0.2"
)

func init() {
	execdriver.RegisterInitFunc(DriverName, func(args *execdriver.InitArgs) error {
		var container *libcontainer.Container
		f, err := os.Open(filepath.Join(args.Root, "container.json"))
		if err != nil {
			return err
		}
		if err := json.NewDecoder(f).Decode(&container); err != nil {
			f.Close()
			return err
		}
		f.Close()

		rootfs, err := os.Getwd()
		if err != nil {
			return err
		}
		syncPipe, err := namespaces.NewSyncPipeFromFd(0, uintptr(args.Pipe))
		if err != nil {
			return err
		}
		if err := namespaces.Init(container, rootfs, args.Console, syncPipe, args.Args); err != nil {
			return err
		}
		return nil
	})
}

type activeContainer struct {
	container *libcontainer.Container
	cmd       *exec.Cmd
}

type driver struct {
	root             string
	initPath         string
	activeContainers map[string]*activeContainer
	sync.Mutex
}

func NewDriver(root, initPath string) (*driver, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}

	// native driver root is at docker_root/execdriver/native. Put apparmor at docker_root
	if err := apparmor.InstallDefaultProfile(); err != nil {
		return nil, err
	}

	return &driver{
		root:             root,
		initPath:         initPath,
		activeContainers: make(map[string]*activeContainer),
	}, nil
}

func (d *driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	// take the Command and populate the libcontainer.Container from it
	container, err := d.createContainer(c)
	if err != nil {
		return -1, err
	}
	d.Lock()
	d.activeContainers[c.ID] = &activeContainer{
		container: container,
		cmd:       &c.Cmd,
	}
	d.Unlock()

	var (
		dataPath = filepath.Join(d.root, c.ID)
		args     = append([]string{c.Entrypoint}, c.Arguments...)
	)
	if err := d.createContainerRoot(c.ID); err != nil {
		return -1, err
	}
	defer d.removeContainerRoot(c.ID)

	if err := d.writeContainerFile(container, c.ID); err != nil {
		return -1, err
	}

	term := getTerminal(c, pipes)

	return namespaces.Exec(container, term, c.Rootfs, dataPath, args, func(container *libcontainer.Container, console, rootfs, dataPath, init string, child *os.File, args []string) *exec.Cmd {
		// we need to join the rootfs because namespaces will setup the rootfs and chroot
		initPath := filepath.Join(c.Rootfs, c.InitPath)

		c.Path = d.initPath
		c.Args = append([]string{
			initPath,
			"-driver", DriverName,
			"-console", console,
			"-pipe", "3",
			"-root", filepath.Join(d.root, c.ID),
			"--",
		}, args...)

		// set this to nil so that when we set the clone flags anything else is reset
		c.SysProcAttr = nil
		system.SetCloneFlags(&c.Cmd, uintptr(namespaces.GetNamespaceFlags(container.Namespaces)))
		c.ExtraFiles = []*os.File{child}

		c.Env = container.Env
		c.Dir = c.Rootfs

		return &c.Cmd
	}, func() {
		if startCallback != nil {
			c.ContainerPid = c.Process.Pid
			startCallback(c)
		}
	})
}

func (d *driver) Kill(p *execdriver.Command, sig int) error {
	return syscall.Kill(p.Process.Pid, syscall.Signal(sig))
}

func (d *driver) Pause(c *execdriver.Command) error {
	active := d.activeContainers[c.ID]
	if active == nil {
		return fmt.Errorf("active container for %s does not exist", c.ID)
	}
	active.container.Cgroups.Freezer = "FROZEN"
	if systemd.UseSystemd() {
		return systemd.Freeze(active.container.Cgroups, active.container.Cgroups.Freezer)
	}
	return fs.Freeze(active.container.Cgroups, active.container.Cgroups.Freezer)
}

func (d *driver) Unpause(c *execdriver.Command) error {
	active := d.activeContainers[c.ID]
	if active == nil {
		return fmt.Errorf("active container for %s does not exist", c.ID)
	}
	active.container.Cgroups.Freezer = "THAWED"
	if systemd.UseSystemd() {
		return systemd.Freeze(active.container.Cgroups, active.container.Cgroups.Freezer)
	}
	return fs.Freeze(active.container.Cgroups, active.container.Cgroups.Freezer)
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
		syscall.Wait4(p.Process.Pid, nil, 0, nil)
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

func (d *driver) GetPidsForContainer(id string) ([]int, error) {
	d.Lock()
	active := d.activeContainers[id]
	d.Unlock()

	if active == nil {
		return nil, fmt.Errorf("active container for %s does not exist", id)
	}
	c := active.container.Cgroups

	if systemd.UseSystemd() {
		return systemd.GetPids(c)
	}
	return fs.GetPids(c)
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
	d.Lock()
	delete(d.activeContainers, id)
	d.Unlock()

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

func getTerminal(c *execdriver.Command, pipes *execdriver.Pipes) namespaces.Terminal {
	var term namespaces.Terminal
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
	return term
}
