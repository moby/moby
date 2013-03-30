package docker

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/rcli"
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

type Container struct {
	root string

	Id string

	Created time.Time

	Path string
	Args []string

	Config *Config
	State  State
	Image  string

	network         *NetworkInterface
	NetworkSettings *NetworkSettings

	SysInitPath string
	cmd         *exec.Cmd
	stdout      *writeBroadcaster
	stderr      *writeBroadcaster
	stdin       io.ReadCloser
	stdinPipe   io.WriteCloser

	ptyStdinMaster  io.Closer
	ptyStdoutMaster io.Closer
	ptyStderrMaster io.Closer

	runtime *Runtime
}

type Config struct {
	Hostname   string
	User       string
	Memory     int64 // Memory limit (in bytes)
	MemorySwap int64 // Total memory usage (memory + swap); set `-1' to disable swap
	Detach     bool
	Ports      []int
	Tty        bool // Attach standard streams to a tty, including stdin if it is not closed.
	OpenStdin  bool // Open stdin
	Env        []string
	Cmd        []string
	Image      string // Name of the image as it was passed by the operator (eg. could be symbolic)
}

func ParseRun(args []string, stdout io.Writer) (*Config, error) {
	cmd := rcli.Subcmd(stdout, "run", "[OPTIONS] IMAGE COMMAND [ARG...]", "Run a command in a new container")
	if len(args) > 0 && args[0] != "--help" {
		cmd.SetOutput(ioutil.Discard)
	}

	flUser := cmd.String("u", "", "Username or UID")
	flDetach := cmd.Bool("d", false, "Detached mode: leave the container running in the background")
	flStdin := cmd.Bool("i", false, "Keep stdin open even if not attached")
	flTty := cmd.Bool("t", false, "Allocate a pseudo-tty")
	flMemory := cmd.Int64("m", 0, "Memory limit (in bytes)")
	var flPorts ports

	cmd.Var(&flPorts, "p", "Map a network port to the container")
	var flEnv ListOpts
	cmd.Var(&flEnv, "e", "Set environment variables")
	if err := cmd.Parse(args); err != nil {
		return nil, err
	}
	parsedArgs := cmd.Args()
	runCmd := []string{}
	image := ""
	if len(parsedArgs) >= 1 {
		image = cmd.Arg(0)
	}
	if len(parsedArgs) > 1 {
		runCmd = parsedArgs[1:]
	}
	config := &Config{
		Ports:     flPorts,
		User:      *flUser,
		Tty:       *flTty,
		OpenStdin: *flStdin,
		Memory:    *flMemory,
		Detach:    *flDetach,
		Env:       flEnv,
		Cmd:       runCmd,
		Image:     image,
	}
	return config, nil
}

type NetworkSettings struct {
	IpAddress   string
	IpPrefixLen int
	Gateway     string
	PortMapping map[string]string
}

func (container *Container) Cmd() *exec.Cmd {
	return container.cmd
}

func (container *Container) When() time.Time {
	return container.Created
}

func (container *Container) FromDisk() error {
	data, err := ioutil.ReadFile(container.jsonPath())
	if err != nil {
		return err
	}
	// Load container settings
	if err := json.Unmarshal(data, container); err != nil {
		return err
	}
	return nil
}

func (container *Container) ToDisk() (err error) {
	data, err := json.Marshal(container)
	if err != nil {
		return
	}
	return ioutil.WriteFile(container.jsonPath(), data, 0666)
}

func (container *Container) generateLXCConfig() error {
	fo, err := os.Create(container.lxcConfigPath())
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
	stdoutMaster, stdoutSlave, err := pty.Open()
	if err != nil {
		return err
	}
	container.ptyStdoutMaster = stdoutMaster
	container.cmd.Stdout = stdoutSlave

	stderrMaster, stderrSlave, err := pty.Open()
	if err != nil {
		return err
	}
	container.ptyStderrMaster = stderrMaster
	container.cmd.Stderr = stderrSlave

	// Copy the PTYs to our broadcasters
	go func() {
		defer container.stdout.Close()
		Debugf("[startPty] Begin of stdout pipe")
		io.Copy(container.stdout, stdoutMaster)
		Debugf("[startPty] End of stdout pipe")
	}()

	go func() {
		defer container.stderr.Close()
		Debugf("[startPty] Begin of stderr pipe")
		io.Copy(container.stderr, stderrMaster)
		Debugf("[startPty] End of stderr pipe")
	}()

	// stdin
	var stdinSlave io.ReadCloser
	if container.Config.OpenStdin {
		stdinMaster, stdinSlave, err := pty.Open()
		if err != nil {
			return err
		}
		container.ptyStdinMaster = stdinMaster
		container.cmd.Stdin = stdinSlave
		// FIXME: The following appears to be broken.
		// "cannot set terminal process group (-1): Inappropriate ioctl for device"
		// container.cmd.SysProcAttr = &syscall.SysProcAttr{Setctty: true, Setsid: true}
		go func() {
			defer container.stdin.Close()
			Debugf("[startPty] Begin of stdin pipe")
			io.Copy(stdinMaster, container.stdin)
			Debugf("[startPty] End of stdin pipe")
		}()
	}
	if err := container.cmd.Start(); err != nil {
		return err
	}
	stdoutSlave.Close()
	stderrSlave.Close()
	if stdinSlave != nil {
		stdinSlave.Close()
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
			Debugf("Begin of stdin pipe [start]")
			io.Copy(stdin, container.stdin)
			Debugf("End of stdin pipe [start]")
		}()
	}
	return container.cmd.Start()
}

func (container *Container) Start() error {
	if err := container.EnsureMounted(); err != nil {
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
		"-f", container.lxcConfigPath(),
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

	container.cmd = exec.Command("lxc-start", params...)

	// Setup environment
	container.cmd.Env = append(
		[]string{
			"HOME=/",
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
		container.Config.Env...,
	)

	// Setup logging of stdout and stderr to disk
	if err := container.runtime.LogToDisk(container.stdout, container.logPath("stdout")); err != nil {
		return err
	}
	if err := container.runtime.LogToDisk(container.stderr, container.logPath("stderr")); err != nil {
		return err
	}

	var err error
	if container.Config.Tty {
		container.cmd.Env = append(
			[]string{"TERM=xterm"},
			container.cmd.Env...,
		)
		err = container.startPty()
	} else {
		err = container.start()
	}
	if err != nil {
		return err
	}
	// FIXME: save state on disk *first*, then converge
	// this way disk state is used as a journal, eg. we can restore after crash etc.
	container.State.setRunning(container.cmd.Process.Pid)
	container.ToDisk()
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

func (container *Container) StderrPipe() (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	container.stderr.AddWriter(writer)
	return newBufReader(reader), nil
}

func (container *Container) allocateNetwork() error {
	iface, err := container.runtime.networkManager.Allocate()
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
	Debugf("Waiting for process")
	if err := container.cmd.Wait(); err != nil {
		// Discard the error as any signals or non 0 returns will generate an error
		Debugf("%s: Process: %s", container.Id, err)
	}
	Debugf("Process finished")

	exitCode := container.cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()

	// Cleanup
	if err := container.releaseNetwork(); err != nil {
		log.Printf("%v: Failed to release network: %v", container.Id, err)
	}
	if err := container.stdout.Close(); err != nil {
		Debugf("%s: Error close stdout: %s", container.Id, err)
	}
	if err := container.stderr.Close(); err != nil {
		Debugf("%s: Error close stderr: %s", container.Id, err)
	}

	if container.ptyStdinMaster != nil {
		if err := container.ptyStdinMaster.Close(); err != nil {
			Debugf("%s: Error close pty stdin master: %s", container.Id, err)
		}
	}
	if container.ptyStdoutMaster != nil {
		if err := container.ptyStdoutMaster.Close(); err != nil {
			Debugf("%s: Error close pty stdout master: %s", container.Id, err)
		}
	}
	if container.ptyStderrMaster != nil {
		if err := container.ptyStderrMaster.Close(); err != nil {
			Debugf("%s: Error close pty stderr master: %s", container.Id, err)
		}
	}

	if err := container.Unmount(); err != nil {
		log.Printf("%v: Failed to umount filesystem: %v", container.Id, err)
	}

	// Re-create a brand new stdin pipe once the container exited
	if container.Config.OpenStdin {
		container.stdin, container.stdinPipe = io.Pipe()
	}

	// Report status back
	container.State.setStopped(exitCode)
	if err := container.ToDisk(); err != nil {
		log.Printf("%s: Failed to dump configuration to the disk: %s", container.Id, err)
	}
}

func (container *Container) kill() error {
	if container.cmd == nil {
		return nil
	}
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
	if output, err := exec.Command("lxc-kill", "-n", container.Id, "15").CombinedOutput(); err != nil {
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

func (container *Container) ExportRw() (Archive, error) {
	return Tar(container.rwPath(), Uncompressed)
}

func (container *Container) Export() (Archive, error) {
	if err := container.EnsureMounted(); err != nil {
		return nil, err
	}
	return Tar(container.RootfsPath(), Uncompressed)
}

func (container *Container) WaitTimeout(timeout time.Duration) error {
	done := make(chan bool)
	go func() {
		container.Wait()
		done <- true
	}()

	select {
	case <-time.After(timeout):
		return fmt.Errorf("Timed Out")
	case <-done:
		return nil
	}
	return nil
}

func (container *Container) EnsureMounted() error {
	if mounted, err := container.Mounted(); err != nil {
		return err
	} else if mounted {
		return nil
	}
	return container.Mount()
}

func (container *Container) Mount() error {
	image, err := container.GetImage()
	if err != nil {
		return err
	}
	return image.Mount(container.RootfsPath(), container.rwPath())
}

func (container *Container) Changes() ([]Change, error) {
	image, err := container.GetImage()
	if err != nil {
		return nil, err
	}
	return image.Changes(container.rwPath())
}

func (container *Container) GetImage() (*Image, error) {
	if container.runtime == nil {
		return nil, fmt.Errorf("Can't get image of unregistered container")
	}
	return container.runtime.graph.Get(container.Image)
}

func (container *Container) Mounted() (bool, error) {
	return Mounted(container.RootfsPath())
}

func (container *Container) Unmount() error {
	return Unmount(container.RootfsPath())
}

func (container *Container) logPath(name string) string {
	return path.Join(container.root, fmt.Sprintf("%s-%s.log", container.Id, name))
}

func (container *Container) ReadLog(name string) (io.Reader, error) {
	return os.Open(container.logPath(name))
}

func (container *Container) jsonPath() string {
	return path.Join(container.root, "config.json")
}

func (container *Container) lxcConfigPath() string {
	return path.Join(container.root, "config.lxc")
}

// This method must be exported to be used from the lxc template
func (container *Container) RootfsPath() string {
	return path.Join(container.root, "rootfs")
}

func (container *Container) rwPath() string {
	return path.Join(container.root, "rw")
}

func validateId(id string) error {
	if id == "" {
		return fmt.Errorf("Invalid empty id")
	}
	return nil
}
