package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/daemon/execdriver"
	"github.com/dotcloud/docker/daemon/graphdriver"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/image"
	"github.com/dotcloud/docker/links"
	"github.com/dotcloud/docker/nat"
	"github.com/dotcloud/docker/pkg/label"
	"github.com/dotcloud/docker/pkg/libcontainer/devices"
	"github.com/dotcloud/docker/pkg/networkfs/etchosts"
	"github.com/dotcloud/docker/pkg/networkfs/resolvconf"
	"github.com/dotcloud/docker/pkg/symlink"
	"github.com/dotcloud/docker/runconfig"
	"github.com/dotcloud/docker/utils"
)

const DefaultPathEnv = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

var (
	ErrNotATTY               = errors.New("The PTY is not a file")
	ErrNoTTY                 = errors.New("No PTY found")
	ErrContainerStart        = errors.New("The container failed to start. Unknown error")
	ErrContainerStartTimeout = errors.New("The container failed to start due to timed out.")
)

type Container struct {
	sync.Mutex
	root   string // Path to the "home" of the container, including metadata.
	basefs string // Path to the graphdriver mountpoint

	ID string

	Created time.Time

	Path string
	Args []string

	Config *runconfig.Config
	State  State
	Image  string

	NetworkSettings *NetworkSettings

	ResolvConfPath string
	HostnamePath   string
	HostsPath      string
	Name           string
	Driver         string
	ExecDriver     string

	command   *execdriver.Command
	stdout    *utils.WriteBroadcaster
	stderr    *utils.WriteBroadcaster
	stdin     io.ReadCloser
	stdinPipe io.WriteCloser

	daemon                   *Daemon
	MountLabel, ProcessLabel string

	waitLock chan struct{}
	Volumes  map[string]string
	// Store rw/ro in a separate structure to preserve reverse-compatibility on-disk.
	// Easier than migrating older container configs :)
	VolumesRW  map[string]bool
	hostConfig *runconfig.HostConfig

	activeLinks map[string]*links.Link
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

	if err := label.ReserveLabel(container.ProcessLabel); err != nil {
		return err
	}
	return container.readHostConfig()
}

func (container *Container) ToDisk() (err error) {
	data, err := json.Marshal(container)
	if err != nil {
		return
	}
	err = ioutil.WriteFile(container.jsonPath(), data, 0666)
	if err != nil {
		return
	}
	return container.WriteHostConfig()
}

func (container *Container) readHostConfig() error {
	container.hostConfig = &runconfig.HostConfig{}
	// If the hostconfig file does not exist, do not read it.
	// (We still have to initialize container.hostConfig,
	// but that's OK, since we just did that above.)
	_, err := os.Stat(container.hostConfigPath())
	if os.IsNotExist(err) {
		return nil
	}
	data, err := ioutil.ReadFile(container.hostConfigPath())
	if err != nil {
		return err
	}
	return json.Unmarshal(data, container.hostConfig)
}

func (container *Container) WriteHostConfig() (err error) {
	data, err := json.Marshal(container.hostConfig)
	if err != nil {
		return
	}
	return ioutil.WriteFile(container.hostConfigPath(), data, 0666)
}

func (container *Container) getResourcePath(path string) string {
	cleanPath := filepath.Join("/", path)
	return filepath.Join(container.basefs, cleanPath)
}

func (container *Container) getRootResourcePath(path string) string {
	cleanPath := filepath.Join("/", path)
	return filepath.Join(container.root, cleanPath)
}

func populateCommand(c *Container, env []string) error {
	var (
		en      *execdriver.Network
		context = make(map[string][]string)
	)
	context["process_label"] = []string{c.GetProcessLabel()}
	context["mount_label"] = []string{c.GetMountLabel()}

	en = &execdriver.Network{
		Mtu:       c.daemon.config.Mtu,
		Interface: nil,
	}

	parts := strings.SplitN(string(c.hostConfig.NetworkMode), ":", 2)
	switch parts[0] {
	case "none":
	case "host":
		en.HostNetworking = true
	case "bridge", "": // empty string to support existing containers
		if !c.Config.NetworkDisabled {
			network := c.NetworkSettings
			en.Interface = &execdriver.NetworkInterface{
				Gateway:     network.Gateway,
				Bridge:      network.Bridge,
				IPAddress:   network.IPAddress,
				IPPrefixLen: network.IPPrefixLen,
			}
		}
	case "container":
		nc, err := c.getNetworkedContainer()
		if err != nil {
			return err
		}
		en.ContainerID = nc.ID
	default:
		return fmt.Errorf("invalid network mode: %s", c.hostConfig.NetworkMode)
	}

	// TODO: this can be removed after lxc-conf is fully deprecated
	mergeLxcConfIntoOptions(c.hostConfig, context)

	resources := &execdriver.Resources{
		Memory:     c.Config.Memory,
		MemorySwap: c.Config.MemorySwap,
		CpuShares:  c.Config.CpuShares,
		Cpuset:     c.Config.Cpuset,
	}
	c.command = &execdriver.Command{
		ID:                 c.ID,
		Privileged:         c.hostConfig.Privileged,
		Rootfs:             c.RootfsPath(),
		InitPath:           "/.dockerinit",
		Entrypoint:         c.Path,
		Arguments:          c.Args,
		WorkingDir:         c.Config.WorkingDir,
		Network:            en,
		Tty:                c.Config.Tty,
		User:               c.Config.User,
		Config:             context,
		Resources:          resources,
		AllowedDevices:     devices.DefaultAllowedDevices,
		AutoCreatedDevices: devices.DefaultAutoCreatedDevices,
	}
	c.command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	c.command.Env = env
	return nil
}

func (container *Container) Start() (err error) {
	container.Lock()
	defer container.Unlock()

	if container.State.IsRunning() {
		return nil
	}
	// if we encounter and error during start we need to ensure that any other
	// setup has been cleaned up properly
	defer func() {
		if err != nil {
			container.cleanup()
		}
	}()

	if err := container.setupContainerDns(); err != nil {
		return err
	}
	if err := container.Mount(); err != nil {
		return err
	}
	if err := container.initializeNetworking(); err != nil {
		return err
	}
	container.verifyDaemonSettings()
	if err := prepareVolumesForContainer(container); err != nil {
		return err
	}
	linkedEnv, err := container.setupLinkedContainers()
	if err != nil {
		return err
	}
	if err := container.setupWorkingDirectory(); err != nil {
		return err
	}
	env := container.createDaemonEnvironment(linkedEnv)
	if err := populateCommand(container, env); err != nil {
		return err
	}
	if err := setupMountsForContainer(container); err != nil {
		return err
	}
	if err := container.startLoggingToDisk(); err != nil {
		return err
	}
	container.waitLock = make(chan struct{})

	return container.waitForStart()
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

// Container.StdinPipe returns a WriteCloser which can be used to feed data
// to the standard input of the container's active process.
// Container.StdoutPipe and Container.StderrPipe each return a ReadCloser
// which can be used to retrieve the standard output (and error) generated
// by the container's active process. The output (and error) are actually
// copied and delivered to all StdoutPipe and StderrPipe consumers, using
// a kind of "broadcaster".

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

func (container *Container) StdoutLogPipe() io.ReadCloser {
	reader, writer := io.Pipe()
	container.stdout.AddWriter(writer, "stdout")
	return utils.NewBufReader(reader)
}

func (container *Container) StderrLogPipe() io.ReadCloser {
	reader, writer := io.Pipe()
	container.stderr.AddWriter(writer, "stderr")
	return utils.NewBufReader(reader)
}

func (container *Container) buildHostnameFile() error {
	container.HostnamePath = container.getRootResourcePath("hostname")
	if container.Config.Domainname != "" {
		return ioutil.WriteFile(container.HostnamePath, []byte(fmt.Sprintf("%s.%s\n", container.Config.Hostname, container.Config.Domainname)), 0644)
	}
	return ioutil.WriteFile(container.HostnamePath, []byte(container.Config.Hostname+"\n"), 0644)
}

func (container *Container) buildHostnameAndHostsFiles(IP string) error {
	if err := container.buildHostnameFile(); err != nil {
		return err
	}

	container.HostsPath = container.getRootResourcePath("hosts")

	extraContent := make(map[string]string)

	children, err := container.daemon.Children(container.Name)
	if err != nil {
		return err
	}

	for linkAlias, child := range children {
		_, alias := path.Split(linkAlias)
		extraContent[alias] = child.NetworkSettings.IPAddress
	}

	return etchosts.Build(container.HostsPath, IP, container.Config.Hostname, container.Config.Domainname, &extraContent)
}

func (container *Container) allocateNetwork() error {
	mode := container.hostConfig.NetworkMode
	if container.Config.NetworkDisabled || mode.IsContainer() || mode.IsHost() {
		return nil
	}

	var (
		env *engine.Env
		err error
		eng = container.daemon.eng
	)

	job := eng.Job("allocate_interface", container.ID)
	if env, err = job.Stdout.AddEnv(); err != nil {
		return err
	}
	if err := job.Run(); err != nil {
		return err
	}

	if container.Config.PortSpecs != nil {
		if err := migratePortMappings(container.Config, container.hostConfig); err != nil {
			return err
		}
		container.Config.PortSpecs = nil
		if err := container.WriteHostConfig(); err != nil {
			return err
		}
	}

	var (
		portSpecs = make(nat.PortSet)
		bindings  = make(nat.PortMap)
	)

	if container.Config.ExposedPorts != nil {
		portSpecs = container.Config.ExposedPorts
	}
	if container.hostConfig.PortBindings != nil {
		bindings = container.hostConfig.PortBindings
	}

	container.NetworkSettings.PortMapping = nil

	for port := range portSpecs {
		if err := container.allocatePort(eng, port, bindings); err != nil {
			return err
		}
	}
	container.WriteHostConfig()

	container.NetworkSettings.Ports = bindings
	container.NetworkSettings.Bridge = env.Get("Bridge")
	container.NetworkSettings.IPAddress = env.Get("IP")
	container.NetworkSettings.IPPrefixLen = env.GetInt("IPPrefixLen")
	container.NetworkSettings.Gateway = env.Get("Gateway")

	return nil
}

func (container *Container) releaseNetwork() {
	if container.Config.NetworkDisabled {
		return
	}
	eng := container.daemon.eng

	eng.Job("release_interface", container.ID).Run()
	container.NetworkSettings = &NetworkSettings{}
}

func (container *Container) monitor(callback execdriver.StartCallback) error {
	var (
		err      error
		exitCode int
	)

	pipes := execdriver.NewPipes(container.stdin, container.stdout, container.stderr, container.Config.OpenStdin)
	exitCode, err = container.daemon.Run(container, pipes, callback)
	if err != nil {
		utils.Errorf("Error running container: %s", err)
	}

	// Cleanup
	container.cleanup()

	// Re-create a brand new stdin pipe once the container exited
	if container.Config.OpenStdin {
		container.stdin, container.stdinPipe = io.Pipe()
	}

	if container.daemon != nil && container.daemon.srv != nil {
		container.daemon.srv.LogEvent("die", container.ID, container.daemon.repositories.ImageName(container.Image))
	}

	close(container.waitLock)

	if container.daemon != nil && container.daemon.srv != nil && container.daemon.srv.IsRunning() {
		container.State.SetStopped(exitCode)

		// FIXME: there is a race condition here which causes this to fail during the unit tests.
		// If another goroutine was waiting for Wait() to return before removing the container's root
		// from the filesystem... At this point it may already have done so.
		// This is because State.setStopped() has already been called, and has caused Wait()
		// to return.
		// FIXME: why are we serializing running state to disk in the first place?
		//log.Printf("%s: Failed to dump configuration to the disk: %s", container.ID, err)
		if err := container.ToDisk(); err != nil {
			utils.Errorf("Error dumping container state to disk: %s\n", err)
		}
	}

	return err
}

func (container *Container) cleanup() {
	container.releaseNetwork()

	// Disable all active links
	if container.activeLinks != nil {
		for _, link := range container.activeLinks {
			link.Disable()
		}
	}
	if container.Config.OpenStdin {
		if err := container.stdin.Close(); err != nil {
			utils.Errorf("%s: Error close stdin: %s", container.ID, err)
		}
	}
	if err := container.stdout.CloseWriters(); err != nil {
		utils.Errorf("%s: Error close stdout: %s", container.ID, err)
	}
	if err := container.stderr.CloseWriters(); err != nil {
		utils.Errorf("%s: Error close stderr: %s", container.ID, err)
	}
	if container.command != nil && container.command.Terminal != nil {
		if err := container.command.Terminal.Close(); err != nil {
			utils.Errorf("%s: Error closing terminal: %s", container.ID, err)
		}
	}

	if err := container.Unmount(); err != nil {
		log.Printf("%v: Failed to umount filesystem: %v", container.ID, err)
	}
}

func (container *Container) KillSig(sig int) error {
	container.Lock()
	defer container.Unlock()

	// We could unpause the container for them rather than returning this error
	if container.State.IsPaused() {
		return fmt.Errorf("Container %s is paused. Unpause the container before stopping", container.ID)
	}

	if !container.State.IsRunning() {
		return nil
	}
	return container.daemon.Kill(container, sig)
}

func (container *Container) Pause() error {
	if container.State.IsPaused() {
		return fmt.Errorf("Container %s is already paused", container.ID)
	}
	if !container.State.IsRunning() {
		return fmt.Errorf("Container %s is not running", container.ID)
	}
	return container.daemon.Pause(container)
}

func (container *Container) Unpause() error {
	if !container.State.IsPaused() {
		return fmt.Errorf("Container %s is not paused", container.ID)
	}
	if !container.State.IsRunning() {
		return fmt.Errorf("Container %s is not running", container.ID)
	}
	return container.daemon.Unpause(container)
}

func (container *Container) Kill() error {
	if !container.State.IsRunning() {
		return nil
	}

	// 1. Send SIGKILL
	if err := container.KillSig(9); err != nil {
		return err
	}

	// 2. Wait for the process to die, in last resort, try to kill the process directly
	if err := container.WaitTimeout(10 * time.Second); err != nil {
		// Ensure that we don't kill ourselves
		if pid := container.State.Pid; pid != 0 {
			log.Printf("Container %s failed to exit within 10 seconds of kill - trying direct SIGKILL", utils.TruncateID(container.ID))
			if err := syscall.Kill(pid, 9); err != nil {
				return err
			}
		}
	}

	container.Wait()
	return nil
}

func (container *Container) Stop(seconds int) error {
	if !container.State.IsRunning() {
		return nil
	}

	// 1. Send a SIGTERM
	if err := container.KillSig(15); err != nil {
		log.Print("Failed to send SIGTERM to the process, force killing")
		if err := container.KillSig(9); err != nil {
			return err
		}
	}

	// 2. Wait for the process to exit on its own
	if err := container.WaitTimeout(time.Duration(seconds) * time.Second); err != nil {
		log.Printf("Container %v failed to exit within %d seconds of SIGTERM - using the force", container.ID, seconds)
		// 3. If it doesn't, then send SIGKILL
		if err := container.Kill(); err != nil {
			container.Wait()
			return err
		}
	}
	return nil
}

func (container *Container) Restart(seconds int) error {
	// Avoid unnecessarily unmounting and then directly mounting
	// the container when the container stops and then starts
	// again
	if err := container.Mount(); err == nil {
		defer container.Unmount()
	}

	if err := container.Stop(seconds); err != nil {
		return err
	}
	return container.Start()
}

// Wait blocks until the container stops running, then returns its exit code.
func (container *Container) Wait() int {
	<-container.waitLock
	return container.State.GetExitCode()
}

func (container *Container) Resize(h, w int) error {
	return container.command.Terminal.Resize(h, w)
}

func (container *Container) ExportRw() (archive.Archive, error) {
	if err := container.Mount(); err != nil {
		return nil, err
	}
	if container.daemon == nil {
		return nil, fmt.Errorf("Can't load storage driver for unregistered container %s", container.ID)
	}
	archive, err := container.daemon.Diff(container)
	if err != nil {
		container.Unmount()
		return nil, err
	}
	return utils.NewReadCloserWrapper(archive, func() error {
			err := archive.Close()
			container.Unmount()
			return err
		}),
		nil
}

func (container *Container) Export() (archive.Archive, error) {
	if err := container.Mount(); err != nil {
		return nil, err
	}

	archive, err := archive.Tar(container.basefs, archive.Uncompressed)
	if err != nil {
		container.Unmount()
		return nil, err
	}
	return utils.NewReadCloserWrapper(archive, func() error {
			err := archive.Close()
			container.Unmount()
			return err
		}),
		nil
}

func (container *Container) WaitTimeout(timeout time.Duration) error {
	done := make(chan bool, 1)
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

func (container *Container) Mount() error {
	return container.daemon.Mount(container)
}

func (container *Container) Changes() ([]archive.Change, error) {
	container.Lock()
	defer container.Unlock()
	return container.daemon.Changes(container)
}

func (container *Container) GetImage() (*image.Image, error) {
	if container.daemon == nil {
		return nil, fmt.Errorf("Can't get image of unregistered container")
	}
	return container.daemon.graph.Get(container.Image)
}

func (container *Container) Unmount() error {
	return container.daemon.Unmount(container)
}

func (container *Container) logPath(name string) string {
	return container.getRootResourcePath(fmt.Sprintf("%s-%s.log", container.ID, name))
}

func (container *Container) ReadLog(name string) (io.Reader, error) {
	return os.Open(container.logPath(name))
}

func (container *Container) hostConfigPath() string {
	return container.getRootResourcePath("hostconfig.json")
}

func (container *Container) jsonPath() string {
	return container.getRootResourcePath("config.json")
}

// This method must be exported to be used from the lxc template
// This directory is only usable when the container is running
func (container *Container) RootfsPath() string {
	return container.basefs
}

func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("Invalid empty id")
	}
	return nil
}

// GetSize, return real size, virtual size
func (container *Container) GetSize() (int64, int64) {
	var (
		sizeRw, sizeRootfs int64
		err                error
		driver             = container.daemon.driver
	)

	if err := container.Mount(); err != nil {
		utils.Errorf("Warning: failed to compute size of container rootfs %s: %s", container.ID, err)
		return sizeRw, sizeRootfs
	}
	defer container.Unmount()

	if differ, ok := container.daemon.driver.(graphdriver.Differ); ok {
		sizeRw, err = differ.DiffSize(container.ID)
		if err != nil {
			utils.Errorf("Warning: driver %s couldn't return diff size of container %s: %s", driver, container.ID, err)
			// FIXME: GetSize should return an error. Not changing it now in case
			// there is a side-effect.
			sizeRw = -1
		}
	} else {
		changes, _ := container.Changes()
		if changes != nil {
			sizeRw = archive.ChangesSize(container.basefs, changes)
		} else {
			sizeRw = -1
		}
	}

	if _, err = os.Stat(container.basefs); err != nil {
		if sizeRootfs, err = utils.TreeSize(container.basefs); err != nil {
			sizeRootfs = -1
		}
	}
	return sizeRw, sizeRootfs
}

func (container *Container) Copy(resource string) (io.ReadCloser, error) {
	if err := container.Mount(); err != nil {
		return nil, err
	}

	var filter []string

	resPath := container.getResourcePath(resource)
	basePath, err := symlink.FollowSymlinkInScope(resPath, container.basefs)
	if err != nil {
		container.Unmount()
		return nil, err
	}

	stat, err := os.Stat(basePath)
	if err != nil {
		container.Unmount()
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

	archive, err := archive.TarFilter(basePath, &archive.TarOptions{
		Compression: archive.Uncompressed,
		Includes:    filter,
	})
	if err != nil {
		container.Unmount()
		return nil, err
	}
	return utils.NewReadCloserWrapper(archive, func() error {
			err := archive.Close()
			container.Unmount()
			return err
		}),
		nil
}

// Returns true if the container exposes a certain port
func (container *Container) Exposes(p nat.Port) bool {
	_, exists := container.Config.ExposedPorts[p]
	return exists
}

func (container *Container) GetPtyMaster() (*os.File, error) {
	ttyConsole, ok := container.command.Terminal.(execdriver.TtyTerminal)
	if !ok {
		return nil, ErrNoTTY
	}
	return ttyConsole.Master(), nil
}

func (container *Container) HostConfig() *runconfig.HostConfig {
	return container.hostConfig
}

func (container *Container) SetHostConfig(hostConfig *runconfig.HostConfig) {
	container.hostConfig = hostConfig
}

func (container *Container) DisableLink(name string) {
	if container.activeLinks != nil {
		if link, exists := container.activeLinks[name]; exists {
			link.Disable()
		} else {
			utils.Debugf("Could not find active link for %s", name)
		}
	}
}

func (container *Container) setupContainerDns() error {
	if container.ResolvConfPath != "" {
		return nil
	}

	var (
		config = container.hostConfig
		daemon = container.daemon
	)

	if config.NetworkMode == "host" {
		container.ResolvConfPath = "/etc/resolv.conf"
		return nil
	}

	resolvConf, err := resolvconf.Get()
	if err != nil {
		return err
	}

	// If custom dns exists, then create a resolv.conf for the container
	if len(config.Dns) > 0 || len(daemon.config.Dns) > 0 || len(config.DnsSearch) > 0 || len(daemon.config.DnsSearch) > 0 {
		var (
			dns       = resolvconf.GetNameservers(resolvConf)
			dnsSearch = resolvconf.GetSearchDomains(resolvConf)
		)
		if len(config.Dns) > 0 {
			dns = config.Dns
		} else if len(daemon.config.Dns) > 0 {
			dns = daemon.config.Dns
		}
		if len(config.DnsSearch) > 0 {
			dnsSearch = config.DnsSearch
		} else if len(daemon.config.DnsSearch) > 0 {
			dnsSearch = daemon.config.DnsSearch
		}
		container.ResolvConfPath = container.getRootResourcePath("resolv.conf")
		return resolvconf.Build(container.ResolvConfPath, dns, dnsSearch)
	} else {
		container.ResolvConfPath = "/etc/resolv.conf"
	}
	return nil
}

func (container *Container) initializeNetworking() error {
	var err error
	if container.hostConfig.NetworkMode.IsHost() {
		container.Config.Hostname, err = os.Hostname()
		if err != nil {
			return err
		}

		parts := strings.SplitN(container.Config.Hostname, ".", 2)
		if len(parts) > 1 {
			container.Config.Hostname = parts[0]
			container.Config.Domainname = parts[1]
		}

		content, err := ioutil.ReadFile("/etc/hosts")
		if os.IsNotExist(err) {
			return container.buildHostnameAndHostsFiles("")
		}
		if err != nil {
			return err
		}

		container.HostsPath = container.getRootResourcePath("hosts")
		return ioutil.WriteFile(container.HostsPath, content, 0644)
	} else if container.hostConfig.NetworkMode.IsContainer() {
		// we need to get the hosts files from the container to join
		nc, err := container.getNetworkedContainer()
		if err != nil {
			return err
		}
		container.HostsPath = nc.HostsPath
		container.ResolvConfPath = nc.ResolvConfPath
		container.Config.Hostname = nc.Config.Hostname
		container.Config.Domainname = nc.Config.Domainname
	} else if container.daemon.config.DisableNetwork {
		container.Config.NetworkDisabled = true
		return container.buildHostnameAndHostsFiles("127.0.1.1")
	} else {
		if err := container.allocateNetwork(); err != nil {
			return err
		}
		return container.buildHostnameAndHostsFiles(container.NetworkSettings.IPAddress)
	}
	return nil
}

// Make sure the config is compatible with the current kernel
func (container *Container) verifyDaemonSettings() {
	if container.Config.Memory > 0 && !container.daemon.sysInfo.MemoryLimit {
		log.Printf("WARNING: Your kernel does not support memory limit capabilities. Limitation discarded.\n")
		container.Config.Memory = 0
	}
	if container.Config.Memory > 0 && !container.daemon.sysInfo.SwapLimit {
		log.Printf("WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.\n")
		container.Config.MemorySwap = -1
	}
	if container.daemon.sysInfo.IPv4ForwardingDisabled {
		log.Printf("WARNING: IPv4 forwarding is disabled. Networking will not work")
	}
}

func (container *Container) setupLinkedContainers() ([]string, error) {
	var (
		env    []string
		daemon = container.daemon
	)
	children, err := daemon.Children(container.Name)
	if err != nil {
		return nil, err
	}

	if len(children) > 0 {
		container.activeLinks = make(map[string]*links.Link, len(children))

		// If we encounter an error make sure that we rollback any network
		// config and ip table changes
		rollback := func() {
			for _, link := range container.activeLinks {
				link.Disable()
			}
			container.activeLinks = nil
		}

		for linkAlias, child := range children {
			if !child.State.IsRunning() {
				return nil, fmt.Errorf("Cannot link to a non running container: %s AS %s", child.Name, linkAlias)
			}

			link, err := links.NewLink(
				container.NetworkSettings.IPAddress,
				child.NetworkSettings.IPAddress,
				linkAlias,
				child.Config.Env,
				child.Config.ExposedPorts,
				daemon.eng)

			if err != nil {
				rollback()
				return nil, err
			}

			container.activeLinks[link.Alias()] = link
			if err := link.Enable(); err != nil {
				rollback()
				return nil, err
			}

			for _, envVar := range link.ToEnv() {
				env = append(env, envVar)
			}
		}
	}
	return env, nil
}

func (container *Container) createDaemonEnvironment(linkedEnv []string) []string {
	// Setup environment
	env := []string{
		"HOME=/",
		"PATH=" + DefaultPathEnv,
		"HOSTNAME=" + container.Config.Hostname,
	}
	if container.Config.Tty {
		env = append(env, "TERM=xterm")
	}
	env = append(env, linkedEnv...)
	// because the env on the container can override certain default values
	// we need to replace the 'env' keys where they match and append anything
	// else.
	env = utils.ReplaceOrAppendEnvValues(env, container.Config.Env)

	return env
}

func (container *Container) setupWorkingDirectory() error {
	if container.Config.WorkingDir != "" {
		container.Config.WorkingDir = path.Clean(container.Config.WorkingDir)

		pthInfo, err := os.Stat(container.getResourcePath(container.Config.WorkingDir))
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			if err := os.MkdirAll(container.getResourcePath(container.Config.WorkingDir), 0755); err != nil {
				return err
			}
		}
		if pthInfo != nil && !pthInfo.IsDir() {
			return fmt.Errorf("Cannot mkdir: %s is not a directory", container.Config.WorkingDir)
		}
	}
	return nil
}

func (container *Container) startLoggingToDisk() error {
	// Setup logging of stdout and stderr to disk
	if err := container.daemon.LogToDisk(container.stdout, container.logPath("json"), "stdout"); err != nil {
		return err
	}
	if err := container.daemon.LogToDisk(container.stderr, container.logPath("json"), "stderr"); err != nil {
		return err
	}
	return nil
}

func (container *Container) waitForStart() error {
	callbackLock := make(chan struct{})
	callback := func(command *execdriver.Command) {
		container.State.SetRunning(command.Pid())
		if command.Tty {
			// The callback is called after the process Start()
			// so we are in the parent process. In TTY mode, stdin/out/err is the PtySlace
			// which we close here.
			if c, ok := command.Stdout.(io.Closer); ok {
				c.Close()
			}
		}
		if err := container.ToDisk(); err != nil {
			utils.Debugf("%s", err)
		}
		close(callbackLock)
	}

	// We use a callback here instead of a goroutine and an chan for
	// syncronization purposes
	cErr := utils.Go(func() error { return container.monitor(callback) })

	// Start should not return until the process is actually running
	select {
	case <-callbackLock:
	case err := <-cErr:
		return err
	}
	return nil
}

func (container *Container) allocatePort(eng *engine.Engine, port nat.Port, bindings nat.PortMap) error {
	binding := bindings[port]
	if container.hostConfig.PublishAllPorts && len(binding) == 0 {
		binding = append(binding, nat.PortBinding{})
	}

	for i := 0; i < len(binding); i++ {
		b := binding[i]

		job := eng.Job("allocate_port", container.ID)
		job.Setenv("HostIP", b.HostIp)
		job.Setenv("HostPort", b.HostPort)
		job.Setenv("Proto", port.Proto())
		job.Setenv("ContainerPort", port.Port())

		portEnv, err := job.Stdout.AddEnv()
		if err != nil {
			return err
		}
		if err := job.Run(); err != nil {
			eng.Job("release_interface", container.ID).Run()
			return err
		}
		b.HostIp = portEnv.Get("HostIP")
		b.HostPort = portEnv.Get("HostPort")

		binding[i] = b
	}
	bindings[port] = binding
	return nil
}

func (container *Container) GetProcessLabel() string {
	// even if we have a process label return "" if we are running
	// in privileged mode
	if container.hostConfig.Privileged {
		return ""
	}
	return container.ProcessLabel
}

func (container *Container) GetMountLabel() string {
	if container.hostConfig.Privileged {
		return ""
	}
	return container.MountLabel
}

func (container *Container) getNetworkedContainer() (*Container, error) {
	parts := strings.SplitN(string(container.hostConfig.NetworkMode), ":", 2)
	switch parts[0] {
	case "container":
		nc := container.daemon.Get(parts[1])
		if nc == nil {
			return nil, fmt.Errorf("no such container to join network: %s", parts[1])
		}
		if !nc.State.IsRunning() {
			return nil, fmt.Errorf("cannot join network of a non running container: %s", parts[1])
		}
		return nc, nil
	default:
		return nil, fmt.Errorf("network mode not set to container")
	}
}
