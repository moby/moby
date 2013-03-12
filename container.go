package docker

import (
	"./fs"
	"encoding/json"
	"errors"
	"github.com/kr/pty"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"
	"time"
)

var sysInitPath string

func init() {
	sysInitPath = SelfPath()
}

type Container struct {
	Id   string
	Root string

	Created time.Time

	Path string
	Args []string

	Config     *Config
	Mountpoint *fs.Mountpoint
	State      *State
	Image      string

	network         *NetworkInterface
	networkManager  *NetworkManager
	NetworkSettings *NetworkSettings

	SysInitPath   string
	lxcConfigPath string
	cmd           *exec.Cmd
	stdout        *writeBroadcaster
	stderr        *writeBroadcaster
	stdin         io.ReadCloser
	stdinPipe     io.WriteCloser

	stdoutLog *os.File
	stderrLog *os.File
}

type Config struct {
	Hostname   string
	User       string
	Memory     int64 // Memory limit (in bytes)
	MemorySwap int64 // Total memory usage (memory + swap); set `-1' to disable swap
	Ports      []int
	Tty        bool // Attach standard streams to a tty, including stdin if it is not closed.
	OpenStdin  bool // Open stdin
}

type NetworkSettings struct {
	IpAddress   string
	IpPrefixLen int
	Gateway     string
	PortMapping map[string]string
}

func createContainer(id string, root string, command string, args []string, image *fs.Image, config *Config, netManager *NetworkManager) (*Container, error) {
	mountpoint, err := image.Mountpoint(path.Join(root, "rootfs"), path.Join(root, "rw"))
	if err != nil {
		return nil, err
	}
	container := &Container{
		Id:              id,
		Root:            root,
		Created:         time.Now(),
		Path:            command,
		Args:            args,
		Config:          config,
		Image:           image.Id,
		Mountpoint:      mountpoint,
		State:           newState(),
		networkManager:  netManager,
		NetworkSettings: &NetworkSettings{},
		SysInitPath:     sysInitPath,
		lxcConfigPath:   path.Join(root, "config.lxc"),
		stdout:          newWriteBroadcaster(),
		stderr:          newWriteBroadcaster(),
	}
	if err := os.Mkdir(root, 0700); err != nil {
		return nil, err
	}
	// Setup logging of stdout and stderr to disk
	if stdoutLog, err := os.OpenFile(path.Join(container.Root, id+"-stdout.log"), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600); err != nil {
		return nil, err
	} else {
		container.stdoutLog = stdoutLog
	}
	if stderrLog, err := os.OpenFile(path.Join(container.Root, id+"-stderr.log"), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600); err != nil {
		return nil, err
	} else {
		container.stderrLog = stderrLog
	}
	if container.Config.OpenStdin {
		container.stdin, container.stdinPipe = io.Pipe()
	} else {
		container.stdinPipe = NopWriteCloser(ioutil.Discard) // Silently drop stdin
	}
	container.stdout.AddWriter(NopWriteCloser(container.stdoutLog))
	container.stderr.AddWriter(NopWriteCloser(container.stderrLog))

	if err := container.save(); err != nil {
		return nil, err
	}
	return container, nil
}

func loadContainer(store *fs.Store, containerPath string, netManager *NetworkManager) (*Container, error) {
	data, err := ioutil.ReadFile(path.Join(containerPath, "config.json"))
	if err != nil {
		return nil, err
	}
	mountpoint, err := store.FetchMountpoint(
		path.Join(containerPath, "rootfs"),
		path.Join(containerPath, "rw"),
	)
	if err != nil {
		return nil, err
	} else if mountpoint == nil {
		return nil, errors.New("Couldn't load container: unregistered mountpoint.")
	}
	container := &Container{
		stdout:          newWriteBroadcaster(),
		stderr:          newWriteBroadcaster(),
		lxcConfigPath:   path.Join(containerPath, "config.lxc"),
		networkManager:  netManager,
		NetworkSettings: &NetworkSettings{},
		Mountpoint:      mountpoint,
	}
	// Load container settings
	if err := json.Unmarshal(data, container); err != nil {
		return nil, err
	}

	// Setup logging of stdout and stderr to disk
	if stdoutLog, err := os.OpenFile(path.Join(container.Root, container.Id+"-stdout.log"), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600); err != nil {
		return nil, err
	} else {
		container.stdoutLog = stdoutLog
	}
	if stderrLog, err := os.OpenFile(path.Join(container.Root, container.Id+"-stderr.log"), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600); err != nil {
		return nil, err
	} else {
		container.stderrLog = stderrLog
	}
	container.stdout.AddWriter(NopWriteCloser(container.stdoutLog))
	container.stderr.AddWriter(NopWriteCloser(container.stderrLog))

	if container.Config.OpenStdin {
		container.stdin, container.stdinPipe = io.Pipe()
	} else {
		container.stdinPipe = NopWriteCloser(ioutil.Discard) // Silently drop stdin
	}
	container.State = newState()
	return container, nil
}

func (container *Container) Cmd() *exec.Cmd {
	return container.cmd
}

func (container *Container) When() time.Time {
	return container.Created
}

func (container *Container) loadUserData() (map[string]string, error) {
	jsonData, err := ioutil.ReadFile(path.Join(container.Root, "userdata.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}
	data := make(map[string]string)
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (container *Container) saveUserData(data map[string]string) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(container.Root, "userdata.json"), jsonData, 0700)
}

func (container *Container) SetUserData(key, value string) error {
	data, err := container.loadUserData()
	if err != nil {
		return err
	}
	data[key] = value
	return container.saveUserData(data)
}

func (container *Container) GetUserData(key string) string {
	data, err := container.loadUserData()
	if err != nil {
		return ""
	}
	if value, exists := data[key]; exists {
		return value
	}
	return ""
}

func (container *Container) save() (err error) {
	data, err := json.Marshal(container)
	if err != nil {
		return
	}
	return ioutil.WriteFile(path.Join(container.Root, "config.json"), data, 0666)
}

func (container *Container) generateLXCConfig() error {
	fo, err := os.Create(container.lxcConfigPath)
	if err != nil {
		return err
	}
	defer fo.Close()

	if err := LxcTemplateCompiled.Execute(fo, container); err != nil {
		return err
	}
	return nil
}

func (container *Container) startPty() error {
	stdout_master, stdout_slave, err := pty.Open()
	if err != nil {
		return err
	}
	container.cmd.Stdout = stdout_slave

	stderr_master, stderr_slave, err := pty.Open()
	if err != nil {
		return err
	}
	container.cmd.Stderr = stderr_slave

	// Copy the PTYs to our broadcasters
	go func() {
		defer container.stdout.Close()
		io.Copy(container.stdout, stdout_master)
	}()

	go func() {
		defer container.stderr.Close()
		io.Copy(container.stderr, stderr_master)
	}()

	// stdin
	var stdin_slave io.ReadCloser
	if container.Config.OpenStdin {
		stdin_master, stdin_slave, err := pty.Open()
		if err != nil {
			return err
		}
		container.cmd.Stdin = stdin_slave
		// FIXME: The following appears to be broken.
		// "cannot set terminal process group (-1): Inappropriate ioctl for device"
		// container.cmd.SysProcAttr = &syscall.SysProcAttr{Setctty: true, Setsid: true}
		go func() {
			defer container.stdin.Close()
			io.Copy(stdin_master, container.stdin)
		}()
	}
	if err := container.cmd.Start(); err != nil {
		return err
	}
	stdout_slave.Close()
	stderr_slave.Close()
	if stdin_slave != nil {
		stdin_slave.Close()
	}
	return nil
}

func (container *Container) start() error {
	container.cmd.Stdout = container.stdout
	container.cmd.Stderr = container.stderr
	if container.Config.OpenStdin {
		stdin, err := container.cmd.StdinPipe()
		if err != nil {
			return err
		}
		go func() {
			defer stdin.Close()
			io.Copy(stdin, container.stdin)
		}()
	}
	return container.cmd.Start()
}

func (container *Container) Start() error {
	if err := container.Mountpoint.EnsureMounted(); err != nil {
		return err
	}
	if err := container.allocateNetwork(); err != nil {
		return err
	}
	if err := container.generateLXCConfig(); err != nil {
		return err
	}
	params := []string{
		"-n", container.Id,
		"-f", container.lxcConfigPath,
		"--",
		"/sbin/init",
	}

	// Networking
	params = append(params, "-g", container.network.Gateway.String())

	// User
	if container.Config.User != "" {
		params = append(params, "-u", container.Config.User)
	}

	// Program
	params = append(params, "--", container.Path)
	params = append(params, container.Args...)

	container.cmd = exec.Command("/usr/bin/lxc-start", params...)

	var err error
	if container.Config.Tty {
		err = container.startPty()
	} else {
		err = container.start()
	}
	if err != nil {
		return err
	}
	container.State.setRunning(container.cmd.Process.Pid)
	container.save()
	go container.monitor()
	return nil
}

func (container *Container) Run() error {
	if err := container.Start(); err != nil {
		return err
	}
	container.Wait()
	return nil
}

func (container *Container) Output() (output []byte, err error) {
	pipe, err := container.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer pipe.Close()
	if err := container.Start(); err != nil {
		return nil, err
	}
	output, err = ioutil.ReadAll(pipe)
	container.Wait()
	return output, err
}

// StdinPipe() returns a pipe connected to the standard input of the container's
// active process.
//
func (container *Container) StdinPipe() (io.WriteCloser, error) {
	return container.stdinPipe, nil
}

func (container *Container) StdoutPipe() (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	container.stdout.AddWriter(writer)
	return newBufReader(reader), nil
}

func (container *Container) StdoutLog() io.Reader {
	r, err := os.Open(container.stdoutLog.Name())
	if err != nil {
		return nil
	}
	return r
}

func (container *Container) StderrPipe() (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	container.stderr.AddWriter(writer)
	return newBufReader(reader), nil
}

func (container *Container) StderrLog() io.Reader {
	r, err := os.Open(container.stderrLog.Name())
	if err != nil {
		return nil
	}
	return r
}

func (container *Container) allocateNetwork() error {
	iface, err := container.networkManager.Allocate()
	if err != nil {
		return err
	}
	container.NetworkSettings.PortMapping = make(map[string]string)
	for _, port := range container.Config.Ports {
		if extPort, err := iface.AllocatePort(port); err != nil {
			iface.Release()
			return err
		} else {
			container.NetworkSettings.PortMapping[strconv.Itoa(port)] = strconv.Itoa(extPort)
		}
	}
	container.network = iface
	container.NetworkSettings.IpAddress = iface.IPNet.IP.String()
	container.NetworkSettings.IpPrefixLen, _ = iface.IPNet.Mask.Size()
	container.NetworkSettings.Gateway = iface.Gateway.String()
	return nil
}

func (container *Container) releaseNetwork() error {
	err := container.network.Release()
	container.network = nil
	container.NetworkSettings = &NetworkSettings{}
	return err
}

func (container *Container) monitor() {
	// Wait for the program to exit
	container.cmd.Wait()
	exitCode := container.cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()

	// Cleanup
	if err := container.releaseNetwork(); err != nil {
		log.Printf("%v: Failed to release network: %v", container.Id, err)
	}
	container.stdout.Close()
	container.stderr.Close()
	if err := container.Mountpoint.Umount(); err != nil {
		log.Printf("%v: Failed to umount filesystem: %v", container.Id, err)
	}

	// Re-create a brand new stdin pipe once the container exited
	if container.Config.OpenStdin {
		container.stdin, container.stdinPipe = io.Pipe()
	}

	// Report status back
	container.State.setStopped(exitCode)
	container.save()
}

func (container *Container) kill() error {
	if err := container.cmd.Process.Kill(); err != nil {
		return err
	}
	// Wait for the container to be actually stopped
	container.Wait()
	return nil
}

func (container *Container) Kill() error {
	if !container.State.Running {
		return nil
	}
	return container.kill()
}

func (container *Container) Stop() error {
	if !container.State.Running {
		return nil
	}

	// 1. Send a SIGTERM
	if output, err := exec.Command("/usr/bin/lxc-kill", "-n", container.Id, "15").CombinedOutput(); err != nil {
		log.Printf(string(output))
		log.Printf("Failed to send SIGTERM to the process, force killing")
		if err := container.Kill(); err != nil {
			return err
		}
	}

	// 2. Wait for the process to exit on its own
	if err := container.WaitTimeout(10 * time.Second); err != nil {
		log.Printf("Container %v failed to exit within 10 seconds of SIGTERM - using the force", container.Id)
		if err := container.Kill(); err != nil {
			return err
		}
	}
	return nil
}

func (container *Container) Restart() error {
	if err := container.Stop(); err != nil {
		return err
	}
	if err := container.Start(); err != nil {
		return err
	}
	return nil
}

// Wait blocks until the container stops running, then returns its exit code.
func (container *Container) Wait() int {

	for container.State.Running {
		container.State.wait()
	}
	return container.State.ExitCode
}

func (container *Container) WaitTimeout(timeout time.Duration) error {
	done := make(chan bool)
	go func() {
		container.Wait()
		done <- true
	}()

	select {
	case <-time.After(timeout):
		return errors.New("Timed Out")
	case <-done:
		return nil
	}
	return nil
}
