package docker

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dotcloud/docker/term"
	"github.com/dotcloud/docker/utils"
	"github.com/kr/pty"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Container struct {
	root string

	ID string

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
	stdout    *utils.WriteBroadcaster
	stderr    *utils.WriteBroadcaster
	stdin     io.ReadCloser
	stdinPipe io.WriteCloser
	ptyMaster io.Closer

	runtime *Runtime

	waitLock chan struct{}
	Volumes  map[string]string
	// Store rw/ro in a separate structure to preserve reverse-compatibility on-disk.
	// Easier than migrating older container configs :)
	VolumesRW map[string]bool
}

type Config struct {
	Hostname        string
	User            string
	Memory          int64 // Memory limit (in bytes)
	MemorySwap      int64 // Total memory usage (memory + swap); set `-1' to disable swap
	CpuShares       int64 // CPU shares (relative weight vs. other containers)
	AttachStdin     bool
	AttachStdout    bool
	AttachStderr    bool
	PortSpecs       []string
	Tty             bool // Attach standard streams to a tty, including stdin if it is not closed.
	OpenStdin       bool // Open stdin
	StdinOnce       bool // If true, close stdin after the 1 attached client disconnects.
	Env             []string
	Cmd             []string
	Dns             []string
	Image           string // Name of the image as it was passed by the operator (eg. could be symbolic)
	Volumes         map[string]struct{}
	VolumesFrom     string
	Entrypoint      []string
	NetworkDisabled bool
}

type HostConfig struct {
	Binds           []string
	ContainerIDFile string
}

type BindMap struct {
	SrcPath string
	DstPath string
	Mode    string
}

func ParseRun(args []string, capabilities *Capabilities) (*Config, *HostConfig, *flag.FlagSet, error) {
	cmd := Subcmd("run", "[OPTIONS] IMAGE [COMMAND] [ARG...]", "Run a command in a new container")
	if len(args) > 0 && args[0] != "--help" {
		cmd.SetOutput(ioutil.Discard)
		cmd.Usage = nil
	}

	flHostname := cmd.String("h", "", "Container host name")
	flUser := cmd.String("u", "", "Username or UID")
	flDetach := cmd.Bool("d", false, "Detached mode: Run container in the background, print new container id")
	flAttach := NewAttachOpts()
	cmd.Var(flAttach, "a", "Attach to stdin, stdout or stderr.")
	flStdin := cmd.Bool("i", false, "Keep stdin open even if not attached")
	flTty := cmd.Bool("t", false, "Allocate a pseudo-tty")
	flMemory := cmd.Int64("m", 0, "Memory limit (in bytes)")
	flContainerIDFile := cmd.String("cidfile", "", "Write the container ID to the file")
	flNetwork := cmd.Bool("n", true, "Enable networking for this container")

	if capabilities != nil && *flMemory > 0 && !capabilities.MemoryLimit {
		//fmt.Fprintf(stdout, "WARNING: Your kernel does not support memory limit capabilities. Limitation discarded.\n")
		*flMemory = 0
	}

	flCpuShares := cmd.Int64("c", 0, "CPU shares (relative weight)")

	var flPorts ListOpts
	cmd.Var(&flPorts, "p", "Expose a container's port to the host (use 'docker port' to see the actual mapping)")

	var flEnv ListOpts
	cmd.Var(&flEnv, "e", "Set environment variables")

	var flDns ListOpts
	cmd.Var(&flDns, "dns", "Set custom dns servers")

	flVolumes := NewPathOpts()
	cmd.Var(flVolumes, "v", "Bind mount a volume (e.g. from the host: -v /host:/container, from docker: -v /container)")

	flVolumesFrom := cmd.String("volumes-from", "", "Mount volumes from the specified container")
	flEntrypoint := cmd.String("entrypoint", "", "Overwrite the default entrypoint of the image")

	if err := cmd.Parse(args); err != nil {
		return nil, nil, cmd, err
	}
	if *flDetach && len(flAttach) > 0 {
		return nil, nil, cmd, fmt.Errorf("Conflicting options: -a and -d")
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

	var binds []string

	// add any bind targets to the list of container volumes
	for bind := range flVolumes {
		arr := strings.Split(bind, ":")
		if len(arr) > 1 {
			dstDir := arr[1]
			flVolumes[dstDir] = struct{}{}
			binds = append(binds, bind)
			delete(flVolumes, bind)
		}
	}

	parsedArgs := cmd.Args()
	runCmd := []string{}
	entrypoint := []string{}
	image := ""
	if len(parsedArgs) >= 1 {
		image = cmd.Arg(0)
	}
	if len(parsedArgs) > 1 {
		runCmd = parsedArgs[1:]
	}
	if *flEntrypoint != "" {
		entrypoint = []string{*flEntrypoint}
	}

	config := &Config{
		Hostname:        *flHostname,
		PortSpecs:       flPorts,
		User:            *flUser,
		Tty:             *flTty,
		NetworkDisabled: !*flNetwork,
		OpenStdin:       *flStdin,
		Memory:          *flMemory,
		CpuShares:       *flCpuShares,
		AttachStdin:     flAttach.Get("stdin"),
		AttachStdout:    flAttach.Get("stdout"),
		AttachStderr:    flAttach.Get("stderr"),
		Env:             flEnv,
		Cmd:             runCmd,
		Dns:             flDns,
		Image:           image,
		Volumes:         flVolumes,
		VolumesFrom:     *flVolumesFrom,
		Entrypoint:      entrypoint,
	}
	hostConfig := &HostConfig{
		Binds:           binds,
		ContainerIDFile: *flContainerIDFile,
	}

	if capabilities != nil && *flMemory > 0 && !capabilities.SwapLimit {
		//fmt.Fprintf(stdout, "WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.\n")
		config.MemorySwap = -1
	}

	// When allocating stdin in attached mode, close stdin at client disconnect
	if config.OpenStdin && config.AttachStdin {
		config.StdinOnce = true
	}
	return config, hostConfig, cmd, nil
}

type PortMapping map[string]string

type NetworkSettings struct {
	IPAddress   string
	IPPrefixLen int
	Gateway     string
	Bridge      string
	PortMapping map[string]PortMapping
}

// String returns a human-readable description of the port mapping defined in the settings
func (settings *NetworkSettings) PortMappingHuman() string {
	var mapping []string
	for private, public := range settings.PortMapping["Tcp"] {
		mapping = append(mapping, fmt.Sprintf("%s->%s", public, private))
	}
	for private, public := range settings.PortMapping["Udp"] {
		mapping = append(mapping, fmt.Sprintf("%s->%s/udp", public, private))
	}
	sort.Strings(mapping)
	return strings.Join(mapping, ", ")
}

// Inject the io.Reader at the given path. Note: do not close the reader
func (container *Container) Inject(file io.Reader, pth string) error {
	// Make sure the directory exists
	if err := os.MkdirAll(path.Join(container.rwPath(), path.Dir(pth)), 0755); err != nil {
		return err
	}
	// FIXME: Handle permissions/already existing dest
	dest, err := os.Create(path.Join(container.rwPath(), pth))
	if err != nil {
		return err
	}
	if _, err := io.Copy(dest, file); err != nil {
		return err
	}
	return nil
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
	// udp broke compat of docker.PortMapping, but it's not used when loading a container, we can skip it
	if err := json.Unmarshal(data, container); err != nil && !strings.Contains(err.Error(), "docker.PortMapping") {
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

func (container *Container) ReadHostConfig() (*HostConfig, error) {
	data, err := ioutil.ReadFile(container.hostConfigPath())
	if err != nil {
		return &HostConfig{}, err
	}
	hostConfig := &HostConfig{}
	if err := json.Unmarshal(data, hostConfig); err != nil {
		return &HostConfig{}, err
	}
	return hostConfig, nil
}

func (container *Container) SaveHostConfig(hostConfig *HostConfig) (err error) {
	data, err := json.Marshal(hostConfig)
	if err != nil {
		return
	}
	return ioutil.WriteFile(container.hostConfigPath(), data, 0666)
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
		utils.Debugf("[startPty] Begin of stdout pipe")
		io.Copy(container.stdout, ptyMaster)
		utils.Debugf("[startPty] End of stdout pipe")
	}()

	// stdin
	if container.Config.OpenStdin {
		container.cmd.Stdin = ptySlave
		container.cmd.SysProcAttr = &syscall.SysProcAttr{Setctty: true, Setsid: true}
		go func() {
			defer container.stdin.Close()
			utils.Debugf("[startPty] Begin of stdin pipe")
			io.Copy(ptyMaster, container.stdin)
			utils.Debugf("[startPty] End of stdin pipe")
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
			utils.Debugf("Begin of stdin pipe [start]")
			io.Copy(stdin, container.stdin)
			utils.Debugf("End of stdin pipe [start]")
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
				utils.Debugf("[start] attach stdin\n")
				defer utils.Debugf("[end] attach stdin\n")
				// No matter what, when stdin is closed (io.Copy unblock), close stdout and stderr
				if container.Config.StdinOnce && !container.Config.Tty {
					defer cStdin.Close()
				} else {
					if cStdout != nil {
						defer cStdout.Close()
					}
					if cStderr != nil {
						defer cStderr.Close()
					}
				}
				if container.Config.Tty {
					_, err = utils.CopyEscapable(cStdin, stdin)
				} else {
					_, err = io.Copy(cStdin, stdin)
				}
				if err != nil {
					utils.Debugf("[error] attach stdin: %s\n", err)
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
				utils.Debugf("[start] attach stdout\n")
				defer utils.Debugf("[end]  attach stdout\n")
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
					utils.Debugf("[error] attach stdout: %s\n", err)
				}
				errors <- err
			}()
		}
	} else {
		go func() {
			if stdinCloser != nil {
				defer stdinCloser.Close()
			}

			if cStdout, err := container.StdoutPipe(); err != nil {
				utils.Debugf("Error stdout pipe")
			} else {
				io.Copy(&utils.NopWriter{}, cStdout)
			}
		}()
	}
	if stderr != nil {
		nJobs += 1
		if p, err := container.StderrPipe(); err != nil {
			errors <- err
		} else {
			cStderr = p
			go func() {
				utils.Debugf("[start] attach stderr\n")
				defer utils.Debugf("[end]  attach stderr\n")
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
					utils.Debugf("[error] attach stderr: %s\n", err)
				}
				errors <- err
			}()
		}
	} else {
		go func() {
			if stdinCloser != nil {
				defer stdinCloser.Close()
			}

			if cStderr, err := container.StderrPipe(); err != nil {
				utils.Debugf("Error stdout pipe")
			} else {
				io.Copy(&utils.NopWriter{}, cStderr)
			}
		}()
	}

	return utils.Go(func() error {
		if cStdout != nil {
			defer cStdout.Close()
		}
		if cStderr != nil {
			defer cStderr.Close()
		}
		// FIXME: how do clean up the stdin goroutine without the unwanted side effect
		// of closing the passed stdin? Add an intermediary io.Pipe?
		for i := 0; i < nJobs; i += 1 {
			utils.Debugf("Waiting for job %d/%d\n", i+1, nJobs)
			if err := <-errors; err != nil {
				utils.Debugf("Job %d returned error %s. Aborting all jobs\n", i+1, err)
				return err
			}
			utils.Debugf("Job %d completed successfully\n", i+1)
		}
		utils.Debugf("All jobs completed successfully\n")
		return nil
	})
}

func (container *Container) Start(hostConfig *HostConfig) error {
	container.State.Lock()
	defer container.State.Unlock()

	if len(hostConfig.Binds) == 0 {
		hostConfig, _ = container.ReadHostConfig()
	}

	if container.State.Running {
		return fmt.Errorf("The container %s is already running.", container.ID)
	}
	if err := container.EnsureMounted(); err != nil {
		return err
	}
	if container.runtime.networkManager.disabled {
		container.Config.NetworkDisabled = true
	} else {
		if err := container.allocateNetwork(); err != nil {
			return err
		}
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

	if !container.runtime.capabilities.IPv4Forwarding {
		log.Printf("WARNING: IPv4 forwarding is disabled. Networking will not work")
	}

	// Create the requested bind mounts
	binds := make(map[string]BindMap)
	// Define illegal container destinations
	illegalDsts := []string{"/", "."}

	for _, bind := range hostConfig.Binds {
		// FIXME: factorize bind parsing in parseBind
		var src, dst, mode string
		arr := strings.Split(bind, ":")
		if len(arr) == 2 {
			src = arr[0]
			dst = arr[1]
			mode = "rw"
		} else if len(arr) == 3 {
			src = arr[0]
			dst = arr[1]
			mode = arr[2]
		} else {
			return fmt.Errorf("Invalid bind specification: %s", bind)
		}

		// Bail if trying to mount to an illegal destination
		for _, illegal := range illegalDsts {
			if dst == illegal {
				return fmt.Errorf("Illegal bind destination: %s", dst)
			}
		}

		bindMap := BindMap{
			SrcPath: src,
			DstPath: dst,
			Mode:    mode,
		}
		binds[path.Clean(dst)] = bindMap
	}

	// FIXME: evaluate volumes-from before individual volumes, so that the latter can override the former.
	// Create the requested volumes volumes
	if container.Volumes == nil || len(container.Volumes) == 0 {
		container.Volumes = make(map[string]string)
		container.VolumesRW = make(map[string]bool)

		for volPath := range container.Config.Volumes {
			volPath = path.Clean(volPath)
			// If an external bind is defined for this volume, use that as a source
			if bindMap, exists := binds[volPath]; exists {
				container.Volumes[volPath] = bindMap.SrcPath
				if strings.ToLower(bindMap.Mode) == "rw" {
					container.VolumesRW[volPath] = true
				}
				// Otherwise create an directory in $ROOT/volumes/ and use that
			} else {
				c, err := container.runtime.volumes.Create(nil, container, "", "", nil)
				if err != nil {
					return err
				}
				srcPath, err := c.layer()
				if err != nil {
					return err
				}
				container.Volumes[volPath] = srcPath
				container.VolumesRW[volPath] = true // RW by default
			}
			// Create the mountpoint
			if err := os.MkdirAll(path.Join(container.RootfsPath(), volPath), 0755); err != nil {
				return nil
			}
		}
	}

	if container.Config.VolumesFrom != "" {
		c := container.runtime.Get(container.Config.VolumesFrom)
		if c == nil {
			return fmt.Errorf("Container %s not found. Impossible to mount its volumes", container.ID)
		}
		for volPath, id := range c.Volumes {
			if _, exists := container.Volumes[volPath]; exists {
				return fmt.Errorf("The requested volume %s overlap one of the volume of the container %s", volPath, c.ID)
			}
			if err := os.MkdirAll(path.Join(container.RootfsPath(), volPath), 0755); err != nil {
				return nil
			}
			container.Volumes[volPath] = id
			if isRW, exists := c.VolumesRW[volPath]; exists {
				container.VolumesRW[volPath] = isRW
			}
		}
	}

	if err := container.generateLXCConfig(); err != nil {
		return err
	}

	params := []string{
		"-n", container.ID,
		"-f", container.lxcConfigPath(),
		"--",
		"/.dockerinit",
	}

	// Networking
	if !container.Config.NetworkDisabled {
		params = append(params, "-g", container.network.Gateway.String())
	}

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
		"-e", "container=lxc",
		"-e", "HOSTNAME="+container.Config.Hostname,
	)

	for _, elem := range container.Config.Env {
		params = append(params, "-e", elem)
	}

	// Program
	params = append(params, "--", container.Path)
	params = append(params, container.Args...)

	container.cmd = exec.Command("lxc-start", params...)

	// Setup logging of stdout and stderr to disk
	if err := container.runtime.LogToDisk(container.stdout, container.logPath("json"), "stdout"); err != nil {
		return err
	}
	if err := container.runtime.LogToDisk(container.stderr, container.logPath("json"), "stderr"); err != nil {
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
	container.SaveHostConfig(hostConfig)
	go container.monitor()
	return nil
}

func (container *Container) Run() error {
	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
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
	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
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
	container.stdout.AddWriter(writer, "")
	return utils.NewBufReader(reader), nil
}

func (container *Container) StderrPipe() (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	container.stderr.AddWriter(writer, "")
	return utils.NewBufReader(reader), nil
}

func (container *Container) allocateNetwork() error {
	if container.Config.NetworkDisabled {
		return nil
	}

	iface, err := container.runtime.networkManager.Allocate()
	if err != nil {
		return err
	}
	container.NetworkSettings.PortMapping = make(map[string]PortMapping)
	container.NetworkSettings.PortMapping["Tcp"] = make(PortMapping)
	container.NetworkSettings.PortMapping["Udp"] = make(PortMapping)
	for _, spec := range container.Config.PortSpecs {
		nat, err := iface.AllocatePort(spec)
		if err != nil {
			iface.Release()
			return err
		}
		proto := strings.Title(nat.Proto)
		backend, frontend := strconv.Itoa(nat.Backend), strconv.Itoa(nat.Frontend)
		container.NetworkSettings.PortMapping[proto][backend] = frontend
	}
	container.network = iface
	container.NetworkSettings.Bridge = container.runtime.networkManager.bridgeIface
	container.NetworkSettings.IPAddress = iface.IPNet.IP.String()
	container.NetworkSettings.IPPrefixLen, _ = iface.IPNet.Mask.Size()
	container.NetworkSettings.Gateway = iface.Gateway.String()
	return nil
}

func (container *Container) releaseNetwork() {
	if container.Config.NetworkDisabled {
		return
	}
	container.network.Release()
	container.network = nil
	container.NetworkSettings = &NetworkSettings{}
}

// FIXME: replace this with a control socket within docker-init
func (container *Container) waitLxc() error {
	for {
		output, err := exec.Command("lxc-info", "-n", container.ID).CombinedOutput()
		if err != nil {
			return err
		}
		if !strings.Contains(string(output), "RUNNING") {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (container *Container) monitor() {
	// Wait for the program to exit
	utils.Debugf("Waiting for process")

	// If the command does not exists, try to wait via lxc
	if container.cmd == nil {
		if err := container.waitLxc(); err != nil {
			utils.Debugf("%s: Process: %s", container.ID, err)
		}
	} else {
		if err := container.cmd.Wait(); err != nil {
			// Discard the error as any signals or non 0 returns will generate an error
			utils.Debugf("%s: Process: %s", container.ID, err)
		}
	}
	utils.Debugf("Process finished")
	if container.runtime != nil && container.runtime.srv != nil {
		container.runtime.srv.LogEvent("die", container.ShortID())
	}
	exitCode := -1
	if container.cmd != nil {
		exitCode = container.cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	}

	// Cleanup
	container.releaseNetwork()
	if container.Config.OpenStdin {
		if err := container.stdin.Close(); err != nil {
			utils.Debugf("%s: Error close stdin: %s", container.ID, err)
		}
	}
	if err := container.stdout.CloseWriters(); err != nil {
		utils.Debugf("%s: Error close stdout: %s", container.ID, err)
	}
	if err := container.stderr.CloseWriters(); err != nil {
		utils.Debugf("%s: Error close stderr: %s", container.ID, err)
	}

	if container.ptyMaster != nil {
		if err := container.ptyMaster.Close(); err != nil {
			utils.Debugf("%s: Error closing Pty master: %s", container.ID, err)
		}
	}

	if err := container.Unmount(); err != nil {
		log.Printf("%v: Failed to umount filesystem: %v", container.ID, err)
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
		//log.Printf("%s: Failed to dump configuration to the disk: %s", container.ID, err)
	}
}

func (container *Container) kill() error {
	if !container.State.Running {
		return nil
	}

	// Sending SIGKILL to the process via lxc
	output, err := exec.Command("lxc-kill", "-n", container.ID, "9").CombinedOutput()
	if err != nil {
		log.Printf("error killing container %s (%s, %s)", container.ID, output, err)
	}

	// 2. Wait for the process to die, in last resort, try to kill the process directly
	if err := container.WaitTimeout(10 * time.Second); err != nil {
		if container.cmd == nil {
			return fmt.Errorf("lxc-kill failed, impossible to kill the container %s", container.ID)
		}
		log.Printf("Container %s failed to exit within 10 seconds of lxc SIGKILL - trying direct SIGKILL", container.ID)
		if err := container.cmd.Process.Kill(); err != nil {
			return err
		}
	}

	// Wait for the container to be actually stopped
	container.Wait()
	return nil
}

func (container *Container) Kill() error {
	container.State.Lock()
	defer container.State.Unlock()
	if !container.State.Running {
		return nil
	}
	return container.kill()
}

func (container *Container) Stop(seconds int) error {
	container.State.Lock()
	defer container.State.Unlock()
	if !container.State.Running {
		return nil
	}

	// 1. Send a SIGTERM
	if output, err := exec.Command("lxc-kill", "-n", container.ID, "15").CombinedOutput(); err != nil {
		log.Print(string(output))
		log.Print("Failed to send SIGTERM to the process, force killing")
		if err := container.kill(); err != nil {
			return err
		}
	}

	// 2. Wait for the process to exit on its own
	if err := container.WaitTimeout(time.Duration(seconds) * time.Second); err != nil {
		log.Printf("Container %v failed to exit within %d seconds of SIGTERM - using the force", container.ID, seconds)
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
	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		return err
	}
	return nil
}

// Wait blocks until the container stops running, then returns its exit code.
func (container *Container) Wait() int {
	<-container.waitLock
	return container.State.ExitCode
}

func (container *Container) Resize(h, w int) error {
	pty, ok := container.ptyMaster.(*os.File)
	if !ok {
		return fmt.Errorf("ptyMaster does not have Fd() method")
	}
	return term.SetWinsize(pty.Fd(), &term.Winsize{Height: uint16(h), Width: uint16(w)})
}

func (container *Container) ExportRw() (Archive, error) {
	return Tar(container.rwPath(), Uncompressed)
}

func (container *Container) RwChecksum() (string, error) {
	rwData, err := Tar(container.rwPath(), Xz)
	if err != nil {
		return "", err
	}
	return utils.HashData(rwData)
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

// ShortID returns a shorthand version of the container's id for convenience.
// A collision with other container shorthands is very unlikely, but possible.
// In case of a collision a lookup with Runtime.Get() will fail, and the caller
// will need to use a langer prefix, or the full-length container Id.
func (container *Container) ShortID() string {
	return utils.TruncateID(container.ID)
}

func (container *Container) logPath(name string) string {
	return path.Join(container.root, fmt.Sprintf("%s-%s.log", container.ID, name))
}

func (container *Container) ReadLog(name string) (io.Reader, error) {
	return os.Open(container.logPath(name))
}

func (container *Container) hostConfigPath() string {
	return path.Join(container.root, "hostconfig.json")
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

func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("Invalid empty id")
	}
	return nil
}

// GetSize, return real size, virtual size
func (container *Container) GetSize() (int64, int64) {
	var sizeRw, sizeRootfs int64

	filepath.Walk(container.rwPath(), func(path string, fileInfo os.FileInfo, err error) error {
		if fileInfo != nil {
			sizeRw += fileInfo.Size()
		}
		return nil
	})

	_, err := os.Stat(container.RootfsPath())
	if err == nil {
		filepath.Walk(container.RootfsPath(), func(path string, fileInfo os.FileInfo, err error) error {
			if fileInfo != nil {
				sizeRootfs += fileInfo.Size()
			}
			return nil
		})
	}
	return sizeRw, sizeRootfs
}

func (container *Container) Copy(resource string) (Archive, error) {
	if err := container.EnsureMounted(); err != nil {
		return nil, err
	}
	var filter []string
	basePath := path.Join(container.RootfsPath(), resource)
	stat, err := os.Stat(basePath)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		d, f := path.Split(basePath)
		basePath = d
		filter = []string{f}
	} else {
		filter = []string{path.Base(basePath)}
		basePath = path.Dir(basePath)
	}
	return TarFilter(basePath, Uncompressed, filter)
}
