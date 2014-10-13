// +build linux
// +build !dockerinit

package libvirt

import (
	"errors"
	"fmt"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/execdriver/lxc"
	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/pkg/rpcfd"
	"github.com/docker/docker/utils"
	"github.com/docker/libcontainer/cgroups"
	"gopkg.in/alexzorin/libvirt-go.v2"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/rpc"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var ErrExec = errors.New("Unsupported: Exec is not supported by the lxc driver")

type driver struct {
	root     string // root path for the driver to use
	version  string
	template *template.Template
	initPath string
	apparmor bool
}

type dockerInit struct {
	command *execdriver.Command
	pipes   *execdriver.Pipes
	socket  *net.UnixConn // needed to prevent rpc FD leak bug
	rpc     *rpc.Client
	rpcLock chan struct{}
}

func (init *dockerInit) Call(method string, args, reply interface{}) error {
	select {
	case <-init.rpcLock:
	case <-time.After(10 * time.Second):
		close(init.rpcLock)
		return fmt.Errorf("timeout waiting for rpc connection")
	}

	if err := init.rpc.Call("DockerInit."+method, args, reply); err != nil {
		return fmt.Errorf("dockerinit rpc call %s failed: %s", method, err)
	}

	return nil
}

func (init *dockerInit) getState() (*StateInfo, error) {
	var stateInfo StateInfo
	var dummy int
	if err := init.Call("GetState", &dummy, &stateInfo); err != nil {
		return nil, err
	}
	return &stateInfo, nil
}

func (init *dockerInit) resume() error {
	var dummy1, dummy2 int
	if err := init.Call("Resume", &dummy1, &dummy2); err != nil {
		return err
	}
	return nil
}

func (init *dockerInit) getPtyMaster() (*os.File, error) {
	var fdRpc rpcfd.RpcFd
	var dummy int
	if err := init.Call("GetPtyMaster", &dummy, &fdRpc); err != nil {
		return nil, err
	}
	return os.NewFile(fdRpc.Fd, "ptyMaster"), nil
}

func (init *dockerInit) getStdin() (*os.File, error) {
	var fdRpc rpcfd.RpcFd
	var dummy int
	if err := init.Call("GetStdin", &dummy, &fdRpc); err != nil {
		return nil, err
	}
	return os.NewFile(fdRpc.Fd, "stdin"), nil
}

func (init *dockerInit) getStdout() (*os.File, error) {
	var fdRpc rpcfd.RpcFd
	var dummy int
	if err := init.Call("GetStdout", &dummy, &fdRpc); err != nil {
		return nil, err
	}
	return os.NewFile(fdRpc.Fd, "stdout"), nil
}

func (init *dockerInit) getStderr() (*os.File, error) {
	var fdRpc rpcfd.RpcFd
	var dummy int
	if err := init.Call("GetStderr", &dummy, &fdRpc); err != nil {
		return nil, err
	}
	return os.NewFile(fdRpc.Fd, "stderr"), nil
}

func (init *dockerInit) getPid() (int, error) {
	var rpcPid rpcfd.RpcPid
	var dummy int
	if err := init.Call("GetPid", &dummy, &rpcPid); err != nil {
		return -1, err
	}
	return int(rpcPid.Pid), nil
}

func (init *dockerInit) signal(signal syscall.Signal) error {
	var dummy int
	if err := init.Call("Signal", &signal, &dummy); err != nil {
		return err
	}
	return nil
}

func (init *dockerInit) newTtyConsole() (*lxc.TtyConsole, error) {

	ptyMaster, err := init.getPtyMaster()
	if err != nil {
		return nil, err
	}

	tty := &lxc.TtyConsole{
		MasterPty: ptyMaster,
	}

	// Attach the pipes
	if init.pipes.Stdin != nil {
		go func() {
			io.Copy(ptyMaster, init.pipes.Stdin)
			ptyMaster.Close()
		}()
	}

	go func() {
		io.Copy(init.pipes.Stdout, ptyMaster)
		ptyMaster.Close()
	}()

	return tty, nil
}

func (init *dockerInit) newStdConsole() (*execdriver.StdConsole, error) {
	std := &execdriver.StdConsole{}

	stdout, err := init.getStdout()
	if err != nil {
		return nil, err
	}

	stderr, err := init.getStderr()
	if err != nil {
		return nil, err
	}

	if init.pipes.Stdin != nil {
		stdin, err := init.getStdin()
		if err != nil {
			return nil, err
		}
		go func() {
			io.Copy(stdin, init.pipes.Stdin)
			stdin.Close()
		}()
	}

	go func() {
		io.Copy(init.pipes.Stdout, stdout)
		stdout.Close()
	}()

	go func() {
		io.Copy(init.pipes.Stderr, stderr)
		stderr.Close()
	}()

	return std, nil
}

func (init *dockerInit) connectConsole() error {

	var (
		term execdriver.Terminal
		err  error
	)

	if init.command.ProcessConfig.Tty {
		term, err = init.newTtyConsole()
		if err != nil {
			return err
		}
	} else {
		term, err = init.newStdConsole()
		if err != nil {
			return err
		}
	}
	init.command.ProcessConfig.Terminal = term
	return nil
}

func (init *dockerInit) wait(callback execdriver.StartCallback, reconnect bool) (int, error) {

	state, err := init.getState()
	if err != nil {
		return -1, err
	}

	if reconnect {
		switch state.State {
		case Running:
			pid, err := init.getPid()
			if err != nil {
				return -1, err
			}

			init.command.ProcessConfig.Process, err = os.FindProcess(pid)
			if err != nil {
				return -1, err
			}

			if err := init.connectConsole(); err != nil {
				return -1, err
			}
		default:
			return -1, fmt.Errorf("can't reconnect to container in state %d", state.State)
		}
	}

	for {
		switch state.State {
		case Initial:
			if err := init.resume(); err != nil {
				return -1, err
			}

		case ConsoleReady:
			if err := init.connectConsole(); err != nil {
				return -1, err
			}

			if err := init.resume(); err != nil {
				return -1, err
			}

		case RunReady:
			if err := init.resume(); err != nil {
				return -1, err
			}

		case Running:
			pid, err := init.getPid()
			if err != nil {
				return -1, err
			}

			init.command.ProcessConfig.Process, err = os.FindProcess(pid)
			if err != nil {
				return -1, err
			}

			if callback != nil {
				callback(&init.command.ProcessConfig, pid)
			}

			if err := init.resume(); err != nil {
				return -1, err
			}

		case Exited:
			// Tell dockerinit it can die, ignore error since the
			// death can disrupt the RPC operation
			init.resume()

			return state.ExitCode, nil

		case FailedToStart:
			// Tell dockerinit it can die, ignore error since the
			// death can disrupt the RPC operation
			init.resume()

			return -1, errors.New(state.Error)

		default:
			return -1, fmt.Errorf("Container is in an unknown state")
		}

		var err error
		state, err = init.getState()
		if err != nil {
			return -1, err
		}
	}

	panic("Unreachable")
}

// Connect to the dockerinit RPC socket
func connectToDockerInit(c *execdriver.Command, p *execdriver.Pipes, reconnect bool) (*dockerInit, error) {
	// We can't connect to the dockerinit RPC socket file directly because
	// the path to it is longer than 108 characters (UNIX_PATH_MAX).
	// Create a temporary symlink to connect to.
	tmpPath, err := ioutil.TempDir("", "docker-rpc.")
	symlink := path.Join(tmpPath, "socket")
	os.Symlink(path.Join(c.Rootfs, SocketPath, RpcSocketName), symlink)
	defer os.RemoveAll(tmpPath)
	address, err := net.ResolveUnixAddr("unix", symlink)
	if err != nil {
		return nil, err
	}

	init := &dockerInit{
		command: c,
		pipes:   p,
		rpcLock: make(chan struct{}),
	}

	// Connect to the dockerinit RPC socket with a 10 second timeout
	for startTime := time.Now(); time.Since(startTime) < 10*time.Second; time.Sleep(10 * time.Millisecond) {
		if init.socket, err = net.DialUnix("unix", nil, address); err == nil {
			init.rpc = rpcfd.NewClient(init.socket)
			break
		}

		if reconnect {
			return nil, fmt.Errorf("container is no longer running")
		}
	}

	if err != nil {
		return nil, fmt.Errorf("socket connection failed: %s", err)
	}

	close(init.rpcLock)

	return init, nil
}

func (init *dockerInit) close() {
	if init.rpc != nil {
		if err := init.rpc.Close(); err != nil {
			// FIXME: Prevent an FD leak by closing the socket
			// directly.  Due to a Go bug, rpc client Close()
			// returns an error if the connection has closed on the
			// other end, and doesn't close the actual socket FD.
			//
			// https://code.google.com/p/go/issues/detail?id=6897
			//
			if err := init.socket.Close(); err != nil {
				log.Errorf("%s: Error closing RPC socket: %s", init.command.ID, err)
			}
		}
		init.rpc = nil
		init.socket = nil
	}
}

func NewDriver(root string, initPath string, apparmor bool) (*driver, error) {
	// test libvirtd connection
	conn, err := libvirt.NewVirConnection("lxc:///")
	if err != nil {
		return nil, err
	}
	defer conn.CloseConnection()

	template, err := getTemplate()
	if err != nil {
		return nil, err
	}
	libVersion, err := conn.GetLibVersion()
	version := "unknown"
	if err == nil {
		major := libVersion / 1000000
		minor := (libVersion % 1000000) / 1000
		micro := libVersion % 1000
		version = fmt.Sprintf("%d.%d.%d", major, minor, micro)
	}

	return &driver{
		root:     root,
		version:  version,
		template: template,
		initPath: initPath,
		apparmor: apparmor,
	}, nil
}

func getDomain(id string) (libvirt.VirConnection, libvirt.VirDomain, error) {
	conn, err := libvirt.NewVirConnection("lxc:///")
	if err != nil {
		return libvirt.VirConnection{}, libvirt.VirDomain{}, err
	}

	domain, err := conn.LookupDomainByName(utils.TruncateID(id))
	if err != nil {
		conn.CloseConnection()
		return libvirt.VirConnection{}, libvirt.VirDomain{}, err
	}
	return conn, domain, nil
}

func (d *driver) Name() string {
	return fmt.Sprintf("%s-%s", DriverName, d.version)
}

func (d *driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, callback execdriver.StartCallback) (int, error) {
	params := []string{}

	c.InitPath = "/" + DockerInitName
	c.Mounts = append(c.Mounts, execdriver.Mount{
		Source:      d.initPath,
		Destination: c.InitPath,
		Writable:    false,
		Private:     true,
	})

	if c.Network.Interface != nil {
		iface := c.Network.Interface
		params = append(params,
			"-g", iface.Gateway,
			"-i", fmt.Sprintf("%s/%d", iface.IPAddress, iface.IPPrefixLen),
			"-mtu", strconv.Itoa(c.Network.Mtu),
		)
	}

	if c.ProcessConfig.User != "" {
		params = append(params, "-u", c.ProcessConfig.User)
	}

	if c.ProcessConfig.Privileged {
		params = append(params, "-privileged")
	}

	if c.WorkingDir != "" {
		params = append(params, "-w", c.WorkingDir)
	}

	if c.ProcessConfig.Tty {
		params = append(params, "-tty")
	}

	if pipes.Stdin != nil {
		params = append(params, "-openstdin")
	}

	params = append(params, "--", c.ProcessConfig.Entrypoint)
	params = append(params, c.ProcessConfig.Arguments...)

	if err := execdriver.GenerateEnvConfig(c, d.root); err != nil {
		return -1, err
	}

	c.ProcessConfig.Entrypoint = c.InitPath
	c.ProcessConfig.Arguments = params

	// Connect to libvirtd
	conn, err := libvirt.NewVirConnection("lxc:///")
	if err != nil {
		return -1, err
	}
	defer conn.CloseConnection()

	// Generate libvirt domain XML file
	filename := path.Join(d.root, "containers", c.ID, "libvirt-lxc-config.xml")

	if err = execdriver.GenerateContainerConfig(d.template, c, d.apparmor, filename); err != nil {
		return -1, err
	}

	// Start up the container
	domain, err := conn.DomainCreateXMLFromFile(filename, libvirt.VIR_DOMAIN_NONE)
	if err != nil {
		return -1, err
	}
	defer domain.Free()

	init, err := connectToDockerInit(c, pipes, false)
	if err != nil {
		return -1, err
	}
	defer init.close()

	return init.wait(callback, false)
}

func (d *driver) Kill(c *execdriver.Command, sig int) error {
	return c.ProcessConfig.Process.Signal(syscall.Signal(sig))
}

func (d *driver) Restore(c *execdriver.Command) (int, error) {
	init, err := connectToDockerInit(c, nil, true)
	if err != nil {
		return -1, err
	}
	defer init.close()

	return init.wait(nil, true)
}

type info struct {
	ID     string
	driver *driver
}

func (i *info) IsRunning() bool {
	id := utils.TruncateID(i.ID)

	conn, err := libvirt.NewVirConnection("lxc:///")
	if err != nil {
		return false
	}
	defer conn.CloseConnection()

	domain, err := conn.LookupDomainByName(id)
	if err != nil {
		return false
	}
	defer domain.Free()

	active, err := domain.IsActive()
	if err != nil {
		return false
	}

	return active
}

func (d *driver) Info(id string) execdriver.Info {
	return &info{
		ID:     id,
		driver: d,
	}
}

func (d *driver) GetPidsForContainer(id string) ([]int, error) {

	// FIXME: ask dockerinit instead and use rpcpid

	id = utils.TruncateID(id)

	conn, err := libvirt.NewVirConnection("lxc:///")
	if err != nil {
		return nil, err
	}
	defer conn.CloseConnection()

	domain, err := conn.LookupDomainByName(id)
	if err != nil {
		return nil, err
	}

	defer domain.Free()

	libvirtPid, err := domain.GetID()
	if err != nil {
		return nil, err
	}

	// Get libvirt_lxc's cgroup
	subsystem := "memory"
	cgroupRoot, err := cgroups.FindCgroupMountpoint(subsystem)
	if err != nil {
		return nil, err
	}
	cgroupFile := filepath.Join("/proc", strconv.Itoa(int(libvirtPid)), "cgroup")
	f, err := os.Open(cgroupFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cgroup, err := cgroups.ParseCgroupFile(subsystem, f)
	if err != nil {
		return nil, err
	}

	// Get other pids in cgroup
	tasksFile := filepath.Join(cgroupRoot, cgroup, "tasks")
	output, err := ioutil.ReadFile(tasksFile)
	if err != nil {
		return nil, err
	}
	pids := []int{}
	// the command in /proc/PID/comm is truncated: no more than 16 chars
	initComm := DockerInitName
	if len(DockerInitName) > 16 {
		initComm = DockerInitName[0:15]
	}
	for _, p := range strings.Split(string(output), "\n") {
		if len(p) == 0 {
			continue
		}
		pid, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("Invalid pid '%s': %s", p, err)
		}
		// skip libvirt_lxc
		if pid == int(libvirtPid) {
			continue
		}

		// The .dockerinit process (pid 1) is an implementation detail,
		// so remove it from the pid list.
		comm, err := ioutil.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
		if err != nil {
			// Ignore any error, the process could have exited
			// already.
			log.Debugf("can't read comm file for pid %d: %s", pid, err)
			continue
		}
		if strings.TrimSpace(string(comm)) == initComm {
			continue
		}

		pids = append(pids, pid)
	}
	return pids, nil
}

func (d *driver) Pause(c *execdriver.Command) error {
	conn, domain, err := getDomain(c.ID)
	if err != nil {
		return err
	}
	defer func() {
		domain.Free()
		conn.CloseConnection()
	}()

	return domain.Suspend()
}

func (d *driver) Unpause(c *execdriver.Command) error {
	conn, domain, err := getDomain(c.ID)
	if err != nil {
		return err
	}
	defer func() {
		domain.Free()
		conn.CloseConnection()
	}()

	return domain.Resume()
}

func (d *driver) Terminate(c *execdriver.Command) error {
	conn, domain, err := getDomain(c.ID)
	if err != nil {
		return err
	}
	defer func() {
		domain.Free()
		conn.CloseConnection()
	}()

	return domain.Destroy()
}

func (d *driver) Clean(id string) error {
	return nil
}

func (d *driver) Exec(c *execdriver.Command, processConfig *execdriver.ProcessConfig, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	return -1, ErrExec
}
