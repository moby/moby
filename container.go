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
	"sort"
	"strconv"
	"strings"
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

	SysInitPath    string
	ResolvConfPath string

	cmd       *exec.Cmd
	stdout    *writeBroadcaster
	stderr    *writeBroadcaster
	stdin     io.ReadCloser
	stdinPipe io.WriteCloser
	ptyMaster io.Closer

	runtime *Runtime

	waitLock chan struct{}
}

type Config struct {
	Hostname     string
	User         string
	Memory       int64 // Memory limit (in bytes)
	MemorySwap   int64 // Total memory usage (memory + swap); set `-1' to disable swap
	AttachStdin  bool
	AttachStdout bool
	AttachStderr bool
	PortSpecs    []string
	Tty          bool // Attach standard streams to a tty, including stdin if it is not closed.
	OpenStdin    bool // Open stdin
	StdinOnce    bool // If true, close stdin after the 1 attached client disconnects.
	Env          []string
	Cmd          []string
	Dns          []string
	Image        string // Name of the image as it was passed by the operator (eg. could be symbolic)
	Volumes      map[string]string
}

func ParseRun(args []string, stdout io.Writer, capabilities *Capabilities) (*Config, error) {
	cmd := rcli.Subcmd(stdout, "run", "[OPTIONS] IMAGE COMMAND [ARG...]", "Run a command in a new container")
	if len(args) > 0 && args[0] != "--help" {
		cmd.SetOutput(ioutil.Discard)
	}

	flHostname := cmd.String("h", "", "Container host name")
	flUser := cmd.String("u", "", "Username or UID")
	flDetach := cmd.Bool("d", false, "Detached mode: leave the container running in the background")
	flAttach := NewAttachOpts()
	cmd.Var(flAttach, "a", "Attach to stdin, stdout or stderr.")
	flStdin := cmd.Bool("i", false, "Keep stdin open even if not attached")
	flTty := cmd.Bool("t", false, "Allocate a pseudo-tty")
	flMemory := cmd.Int64("m", 0, "Memory limit (in bytes)")

	if *flMemory > 0 && !capabilities.MemoryLimit {
		fmt.Fprintf(stdout, "WARNING: Your kernel does not support memory limit capabilities. Limitation discarded.\n")
		*flMemory = 0
	}

	var flPorts ListOpts
	cmd.Var(&flPorts, "p", "Expose a container's port to the host (use 'docker port' to see the actual mapping)")

	var flEnv ListOpts
	cmd.Var(&flEnv, "e", "Set environment variables")

	var flDns ListOpts
	cmd.Var(&flDns, "dns", "Set custom dns servers")

	flVolumes := NewPathOpts()
	cmd.Var(flVolumes, "v", "Attach a data volume")

	if err := cmd.Parse(args); err != nil {
		return nil, err
	}
	if *flDetach && len(flAttach) > 0 {
		return nil, fmt.Errorf("Conflicting options: -a and -d")
	}
	// If neither -d or -a are set, attach to everything by default
	if len(flAttach) == 0 && !*flDetach {
		if !*flDetach {
			flAttach.Set("stdout")
			flAttach.Set("stderr")
			if *flStdin {
				flAttach.Set("stdin")
			}
		}
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
		Hostname:     *flHostname,
		PortSpecs:    flPorts,
		User:         *flUser,
		Tty:          *flTty,
		OpenStdin:    *flStdin,
		Memory:       *flMemory,
		AttachStdin:  flAttach.Get("stdin"),
		AttachStdout: flAttach.Get("stdout"),
		AttachStderr: flAttach.Get("stderr"),
		Env:          flEnv,
		Cmd:          runCmd,
		Dns:          flDns,
		Image:        image,
		Volumes:      flVolumes,
	}

	if *flMemory > 0 && !capabilities.SwapLimit {
		fmt.Fprintf(stdout, "WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.\n")
		config.MemorySwap = -1
	}

	// When allocating stdin in attached mode, close stdin at client disconnect
	if config.OpenStdin && config.AttachStdin {
		config.StdinOnce = true
	}
	return config, nil
}

type NetworkSettings struct {
	IpAddress   string
	IpPrefixLen int
	Gateway     string
	Bridge      string
	PortMapping map[string]string
}

// String returns a human-readable description of the port mapping defined in the settings
func (settings *NetworkSettings) PortMappingHuman() string {
	var mapping []string
	for private, public := range settings.PortMapping {
		mapping = append(mapping, fmt.Sprintf("%s->%s", public, private))
	}
	sort.Strings(mapping)
	return strings.Join(mapping, ", ")
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
	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		return err
	}
	container.ptyMaster = ptyMaster
	container.cmd.Stdout = ptySlave
	container.cmd.Stderr = ptySlave

	// Copy the PTYs to our broadcasters
	go func() {
		defer container.stdout.CloseWriters()
		Debugf("[startPty] Begin of stdout pipe")
		io.Copy(container.stdout, ptyMaster)
		Debugf("[startPty] End of stdout pipe")
	}()

	// stdin
	if container.Config.OpenStdin {
		container.cmd.Stdin = ptySlave
		container.cmd.SysProcAttr = &syscall.SysProcAttr{Setctty: true, Setsid: true}
		go func() {
			defer container.stdin.Close()
			Debugf("[startPty] Begin of stdin pipe")
			io.Copy(ptyMaster, container.stdin)
			Debugf("[startPty] End of stdin pipe")
		}()
	}
	if err := container.cmd.Start(); err != nil {
		return err
	}
	ptySlave.Close()
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

func (container *Container) Attach(stdin io.ReadCloser, stdinCloser io.Closer, stdout io.Writer, stderr io.Writer) chan error {
	var cStdout, cStderr io.ReadCloser

	var nJobs int
	errors := make(chan error, 3)
	if stdin != nil && container.Config.OpenStdin {
		nJobs += 1
		if cStdin, err := container.StdinPipe(); err != nil {
			errors <- err
		} else {
			go func() {
				Debugf("[start] attach stdin\n")
				defer Debugf("[end] attach stdin\n")
				// No matter what, when stdin is closed (io.Copy unblock), close stdout and stderr
				if cStdout != nil {
					defer cStdout.Close()
				}
				if cStderr != nil {
					defer cStderr.Close()
				}
				if container.Config.StdinOnce && !container.Config.Tty {
					defer cStdin.Close()
				}
				if container.Config.Tty {
					_, err = CopyEscapable(cStdin, stdin)
				} else {
					_, err = io.Copy(cStdin, stdin)
				}
				if err != nil {
					Debugf("[error] attach stdin: %s\n", err)
				}
				// Discard error, expecting pipe error
				errors <- nil
			}()
		}
	}
	if stdout != nil {
		nJobs += 1
		if p, err := container.StdoutPipe(); err != nil {
			errors <- err
		} else {
			cStdout = p
			go func() {
				Debugf("[start] attach stdout\n")
				defer Debugf("[end]  attach stdout\n")
				// If we are in StdinOnce mode, then close stdin
				if container.Config.StdinOnce {
					if stdin != nil {
						defer stdin.Close()
					}
					if stdinCloser != nil {
						defer stdinCloser.Close()
					}
				}
				_, err := io.Copy(stdout, cStdout)
				if err != nil {
					Debugf("[error] attach stdout: %s\n", err)
				}
				errors <- err
			}()
		}
	}
	if stderr != nil {
		nJobs += 1
		if p, err := container.StderrPipe(); err != nil {
			errors <- err
		} else {
			cStderr = p
			go func() {
				Debugf("[start] attach stderr\n")
				defer Debugf("[end]  attach stderr\n")
				// If we are in StdinOnce mode, then close stdin
				if container.Config.StdinOnce {
					if stdin != nil {
						defer stdin.Close()
					}
					if stdinCloser != nil {
						defer stdinCloser.Close()
					}
				}
				_, err := io.Copy(stderr, cStderr)
				if err != nil {
					Debugf("[error] attach stderr: %s\n", err)
				}
				errors <- err
			}()
		}
	}
	return Go(func() error {
		if cStdout != nil {
			defer cStdout.Close()
		}
		if cStderr != nil {
			defer cStderr.Close()
		}
		// FIXME: how do clean up the stdin goroutine without the unwanted side effect
		// of closing the passed stdin? Add an intermediary io.Pipe?
		for i := 0; i < nJobs; i += 1 {
			Debugf("Waiting for job %d/%d\n", i+1, nJobs)
			if err := <-errors; err != nil {
				Debugf("Job %d returned error %s. Aborting all jobs\n", i+1, err)
				return err
			}
			Debugf("Job %d completed successfully\n", i+1)
		}
		Debugf("All jobs completed successfully\n")
		return nil
	})
}

func (container *Container) Start() error {
	container.State.lock()
	defer container.State.unlock()

	if container.State.Running {
		return fmt.Errorf("The container %s is already running.", container.Id)
	}
	if err := container.EnsureMounted(); err != nil {
		return err
	}
	if err := container.allocateNetwork(); err != nil {
		return err
	}

	// Make sure the config is compatible with the current kernel
	if container.Config.Memory > 0 && !container.runtime.capabilities.MemoryLimit {
		log.Printf("WARNING: Your kernel does not support memory limit capabilities. Limitation discarded.\n")
		container.Config.Memory = 0
	}
	if container.Config.Memory > 0 && !container.runtime.capabilities.SwapLimit {
		log.Printf("WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.\n")
		container.Config.MemorySwap = -1
	}

	// Create the requested volumes volumes
	for volPath := range container.Config.Volumes {
		if c, err := container.runtime.volumes.Create(nil, container, ""); err != nil {
			return err
		} else {
			if err := os.MkdirAll(path.Join(container.RootfsPath(), volPath), 0755); err != nil {
				return nil
			}
			root, err := c.root()
			if err != nil {
				return err
			}
			container.Config.Volumes[volPath] = root
		}
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

	if container.Config.Tty {
		params = append(params, "-e", "TERM=xterm")
	}

	// Setup environment
	params = append(params,
		"-e", "HOME=/",
		"-e", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	)

	for _, elem := range container.Config.Env {
		params = append(params, "-e", elem)
	}

	// Program
	params = append(params, "--", container.Path)
	params = append(params, container.Args...)

	container.cmd = exec.Command("lxc-start", params...)

	// Setup logging of stdout and stderr to disk
	if err := container.runtime.LogToDisk(container.stdout, container.logPath("stdout")); err != nil {
		return err
	}
	if err := container.runtime.LogToDisk(container.stderr, container.logPath("stderr")); err != nil {
		return err
	}

	var err error
	if container.Config.Tty {
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

	// Init the lock
	container.waitLock = make(chan struct{})

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
	for _, spec := range container.Config.PortSpecs {
		if nat, err := iface.AllocatePort(spec); err != nil {
			iface.Release()
			return err
		} else {
			container.NetworkSettings.PortMapping[strconv.Itoa(nat.Backend)] = strconv.Itoa(nat.Frontend)
		}
	}
	container.network = iface
	container.NetworkSettings.Bridge = container.runtime.networkManager.bridgeIface
	container.NetworkSettings.IpAddress = iface.IPNet.IP.String()
	container.NetworkSettings.IpPrefixLen, _ = iface.IPNet.Mask.Size()
	container.NetworkSettings.Gateway = iface.Gateway.String()
	return nil
}

func (container *Container) releaseNetwork() {
	container.network.Release()
	container.network = nil
	container.NetworkSettings = &NetworkSettings{}
}

// FIXME: replace this with a control socket within docker-init
func (container *Container) waitLxc() error {
	for {
		if output, err := exec.Command("lxc-info", "-n", container.Id).CombinedOutput(); err != nil {
			return err
		} else {
			if !strings.Contains(string(output), "RUNNING") {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

func (container *Container) monitor() {
	// Wait for the program to exit
	Debugf("Waiting for process")

	// If the command does not exists, try to wait via lxc
	if container.cmd == nil {
		if err := container.waitLxc(); err != nil {
			Debugf("%s: Process: %s", container.Id, err)
		}
	} else {
		if err := container.cmd.Wait(); err != nil {
			// Discard the error as any signals or non 0 returns will generate an error
			Debugf("%s: Process: %s", container.Id, err)
		}
	}
	Debugf("Process finished")

	var exitCode int = -1
	if container.cmd != nil {
		exitCode = container.cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	}

	// Cleanup
	container.releaseNetwork()
	if container.Config.OpenStdin {
		if err := container.stdin.Close(); err != nil {
			Debugf("%s: Error close stdin: %s", container.Id, err)
		}
	}
	if err := container.stdout.CloseWriters(); err != nil {
		Debugf("%s: Error close stdout: %s", container.Id, err)
	}
	if err := container.stderr.CloseWriters(); err != nil {
		Debugf("%s: Error close stderr: %s", container.Id, err)
	}

	if container.ptyMaster != nil {
		if err := container.ptyMaster.Close(); err != nil {
			Debugf("%s: Error closing Pty master: %s", container.Id, err)
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

	// Release the lock
	close(container.waitLock)

	if err := container.ToDisk(); err != nil {
		// FIXME: there is a race condition here which causes this to fail during the unit tests.
		// If another goroutine was waiting for Wait() to return before removing the container's root
		// from the filesystem... At this point it may already have done so.
		// This is because State.setStopped() has already been called, and has caused Wait()
		// to return.
		// FIXME: why are we serializing running state to disk in the first place?
		//log.Printf("%s: Failed to dump configuration to the disk: %s", container.Id, err)
	}
}

func (container *Container) kill() error {
	if !container.State.Running {
		return nil
	}

	// Sending SIGKILL to the process via lxc
	output, err := exec.Command("lxc-kill", "-n", container.Id, "9").CombinedOutput()
	if err != nil {
		log.Printf("error killing container %s (%s, %s)", container.Id, output, err)
	}

	// 2. Wait for the process to die, in last resort, try to kill the process directly
	if err := container.WaitTimeout(10 * time.Second); err != nil {
		if container.cmd == nil {
			return fmt.Errorf("lxc-kill failed, impossible to kill the container %s", container.Id)
		}
		log.Printf("Container %s failed to exit within 10 seconds of lxc SIGKILL - trying direct SIGKILL", container.Id)
		if err := container.cmd.Process.Kill(); err != nil {
			return err
		}
	}

	// Wait for the container to be actually stopped
	container.Wait()
	return nil
}

func (container *Container) Kill() error {
	container.State.lock()
	defer container.State.unlock()
	if !container.State.Running {
		return nil
	}
	return container.kill()
}

func (container *Container) Stop(seconds int) error {
	container.State.lock()
	defer container.State.unlock()
	if !container.State.Running {
		return nil
	}

	// 1. Send a SIGTERM
	if output, err := exec.Command("lxc-kill", "-n", container.Id, "15").CombinedOutput(); err != nil {
		log.Print(string(output))
		log.Print("Failed to send SIGTERM to the process, force killing")
		if err := container.kill(); err != nil {
			return err
		}
	}

	// 2. Wait for the process to exit on its own
	if err := container.WaitTimeout(time.Duration(seconds) * time.Second); err != nil {
		log.Printf("Container %v failed to exit within %d seconds of SIGTERM - using the force", container.Id, seconds)
		if err := container.kill(); err != nil {
			return err
		}
	}
	return nil
}

func (container *Container) Restart(seconds int) error {
	if err := container.Stop(seconds); err != nil {
		return err
	}
	if err := container.Start(); err != nil {
		return err
	}
	return nil
}

// Wait blocks until the container stops running, then returns its exit code.
func (container *Container) Wait() int {
	<-container.waitLock
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
	panic("unreachable")
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

// ShortId returns a shorthand version of the container's id for convenience.
// A collision with other container shorthands is very unlikely, but possible.
// In case of a collision a lookup with Runtime.Get() will fail, and the caller
// will need to use a langer prefix, or the full-length container Id.
func (container *Container) ShortId() string {
	return TruncateId(container.Id)
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
