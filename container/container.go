package container

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	containertypes "github.com/docker/docker/api/types/container"
	mounttypes "github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/container/stream"
	"github.com/docker/docker/daemon/exec"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/jsonfilelog"
	"github.com/docker/docker/daemon/network"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/restartmanager"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/volume"
	"github.com/docker/go-connections/nat"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/types"
	agentexec "github.com/docker/swarmkit/agent/exec"
	"github.com/opencontainers/runc/libcontainer/label"
)

const configFileName = "config.v2.json"

const (
	// DefaultStopTimeout is the timeout (in seconds) for the syscall signal used to stop a container.
	DefaultStopTimeout = 10
)

var (
	errInvalidEndpoint = fmt.Errorf("invalid endpoint while building port map info")
	errInvalidNetwork  = fmt.Errorf("invalid network settings while building port map info")
)

// CommonContainer holds the fields for a container which are
// applicable across all platforms supported by the daemon.
type CommonContainer struct {
	StreamConfig *stream.Config
	// embed for Container to support states directly.
	*State          `json:"State"` // Needed for Engine API version <= 1.11
	Root            string         `json:"-"` // Path to the "home" of the container, including metadata.
	BaseFS          string         `json:"-"` // Path to the graphdriver mountpoint
	RWLayer         layer.RWLayer  `json:"-"`
	ID              string
	Created         time.Time
	Managed         bool
	Path            string
	Args            []string
	Config          *containertypes.Config
	ImageID         image.ID `json:"Image"`
	NetworkSettings *network.Settings
	LogPath         string
	Name            string
	Driver          string
	// MountLabel contains the options for the 'mount' command
	MountLabel             string
	ProcessLabel           string
	RestartCount           int
	HasBeenStartedBefore   bool
	HasBeenManuallyStopped bool // used for unless-stopped restart policy
	MountPoints            map[string]*volume.MountPoint
	HostConfig             *containertypes.HostConfig `json:"-"` // do not serialize the host config in the json, otherwise we'll make the container unportable
	ExecCommands           *exec.Store                `json:"-"`
	SecretStore            agentexec.SecretGetter     `json:"-"`
	SecretReferences       []*swarmtypes.SecretReference
	// logDriver for closing
	LogDriver      logger.Logger  `json:"-"`
	LogCopier      *logger.Copier `json:"-"`
	restartManager restartmanager.RestartManager
	attachContext  *attachContext
}

// NewBaseContainer creates a new container with its
// basic configuration.
func NewBaseContainer(id, root string) *Container {
	return &Container{
		CommonContainer: CommonContainer{
			ID:            id,
			State:         NewState(),
			ExecCommands:  exec.NewStore(),
			Root:          root,
			MountPoints:   make(map[string]*volume.MountPoint),
			StreamConfig:  stream.NewConfig(),
			attachContext: &attachContext{},
		},
	}
}

// FromDisk loads the container configuration stored in the host.
func (container *Container) FromDisk() error {
	pth, err := container.ConfigPath()
	if err != nil {
		return err
	}

	jsonSource, err := os.Open(pth)
	if err != nil {
		return err
	}
	defer jsonSource.Close()

	dec := json.NewDecoder(jsonSource)

	// Load container settings
	if err := dec.Decode(container); err != nil {
		return err
	}

	if err := label.ReserveLabel(container.ProcessLabel); err != nil {
		return err
	}
	return container.readHostConfig()
}

// ToDisk saves the container configuration on disk.
func (container *Container) ToDisk() error {
	pth, err := container.ConfigPath()
	if err != nil {
		return err
	}

	jsonSource, err := ioutils.NewAtomicFileWriter(pth, 0644)
	if err != nil {
		return err
	}
	defer jsonSource.Close()

	enc := json.NewEncoder(jsonSource)

	// Save container settings
	if err := enc.Encode(container); err != nil {
		return err
	}

	return container.WriteHostConfig()
}

// ToDiskLocking saves the container configuration on disk in a thread safe way.
func (container *Container) ToDiskLocking() error {
	container.Lock()
	err := container.ToDisk()
	container.Unlock()
	return err
}

// readHostConfig reads the host configuration from disk for the container.
func (container *Container) readHostConfig() error {
	container.HostConfig = &containertypes.HostConfig{}
	// If the hostconfig file does not exist, do not read it.
	// (We still have to initialize container.HostConfig,
	// but that's OK, since we just did that above.)
	pth, err := container.HostConfigPath()
	if err != nil {
		return err
	}

	f, err := os.Open(pth)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&container.HostConfig); err != nil {
		return err
	}

	container.InitDNSHostConfig()

	return nil
}

// WriteHostConfig saves the host configuration on disk for the container.
func (container *Container) WriteHostConfig() error {
	pth, err := container.HostConfigPath()
	if err != nil {
		return err
	}

	f, err := ioutils.NewAtomicFileWriter(pth, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(&container.HostConfig)
}

// SetupWorkingDirectory sets up the container's working directory as set in container.Config.WorkingDir
func (container *Container) SetupWorkingDirectory(rootUID, rootGID int) error {
	if container.Config.WorkingDir == "" {
		return nil
	}

	container.Config.WorkingDir = filepath.Clean(container.Config.WorkingDir)

	// If can't mount container FS at this point (e.g. Hyper-V Containers on
	// Windows) bail out now with no action.
	if !container.canMountFS() {
		return nil
	}

	pth, err := container.GetResourcePath(container.Config.WorkingDir)
	if err != nil {
		return err
	}

	if err := idtools.MkdirAllNewAs(pth, 0755, rootUID, rootGID); err != nil {
		pthInfo, err2 := os.Stat(pth)
		if err2 == nil && pthInfo != nil && !pthInfo.IsDir() {
			return fmt.Errorf("Cannot mkdir: %s is not a directory", container.Config.WorkingDir)
		}

		return err
	}

	return nil
}

// GetResourcePath evaluates `path` in the scope of the container's BaseFS, with proper path
// sanitisation. Symlinks are all scoped to the BaseFS of the container, as
// though the container's BaseFS was `/`.
//
// The BaseFS of a container is the host-facing path which is bind-mounted as
// `/` inside the container. This method is essentially used to access a
// particular path inside the container as though you were a process in that
// container.
//
// NOTE: The returned path is *only* safely scoped inside the container's BaseFS
//       if no component of the returned path changes (such as a component
//       symlinking to a different path) between using this method and using the
//       path. See symlink.FollowSymlinkInScope for more details.
func (container *Container) GetResourcePath(path string) (string, error) {
	// IMPORTANT - These are paths on the OS where the daemon is running, hence
	// any filepath operations must be done in an OS agnostic way.

	cleanPath := cleanResourcePath(path)
	r, e := symlink.FollowSymlinkInScope(filepath.Join(container.BaseFS, cleanPath), container.BaseFS)

	// Log this here on the daemon side as there's otherwise no indication apart
	// from the error being propagated all the way back to the client. This makes
	// debugging significantly easier and clearly indicates the error comes from the daemon.
	if e != nil {
		logrus.Errorf("Failed to FollowSymlinkInScope BaseFS %s cleanPath %s path %s %s\n", container.BaseFS, cleanPath, path, e)
	}
	return r, e
}

// GetRootResourcePath evaluates `path` in the scope of the container's root, with proper path
// sanitisation. Symlinks are all scoped to the root of the container, as
// though the container's root was `/`.
//
// The root of a container is the host-facing configuration metadata directory.
// Only use this method to safely access the container's `container.json` or
// other metadata files. If in doubt, use container.GetResourcePath.
//
// NOTE: The returned path is *only* safely scoped inside the container's root
//       if no component of the returned path changes (such as a component
//       symlinking to a different path) between using this method and using the
//       path. See symlink.FollowSymlinkInScope for more details.
func (container *Container) GetRootResourcePath(path string) (string, error) {
	// IMPORTANT - These are paths on the OS where the daemon is running, hence
	// any filepath operations must be done in an OS agnostic way.
	cleanPath := filepath.Join(string(os.PathSeparator), path)
	return symlink.FollowSymlinkInScope(filepath.Join(container.Root, cleanPath), container.Root)
}

// ExitOnNext signals to the monitor that it should not restart the container
// after we send the kill signal.
func (container *Container) ExitOnNext() {
	container.RestartManager().Cancel()
}

// HostConfigPath returns the path to the container's JSON hostconfig
func (container *Container) HostConfigPath() (string, error) {
	return container.GetRootResourcePath("hostconfig.json")
}

// ConfigPath returns the path to the container's JSON config
func (container *Container) ConfigPath() (string, error) {
	return container.GetRootResourcePath(configFileName)
}

// CheckpointDir returns the directory checkpoints are stored in
func (container *Container) CheckpointDir() string {
	return filepath.Join(container.Root, "checkpoints")
}

// StartLogger starts a new logger driver for the container.
func (container *Container) StartLogger() (logger.Logger, error) {
	cfg := container.HostConfig.LogConfig
	c, err := logger.GetLogDriver(cfg.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to get logging factory: %v", err)
	}
	info := logger.Info{
		Config:              cfg.Config,
		ContainerID:         container.ID,
		ContainerName:       container.Name,
		ContainerEntrypoint: container.Path,
		ContainerArgs:       container.Args,
		ContainerImageID:    container.ImageID.String(),
		ContainerImageName:  container.Config.Image,
		ContainerCreated:    container.Created,
		ContainerEnv:        container.Config.Env,
		ContainerLabels:     container.Config.Labels,
		DaemonName:          "docker",
	}

	// Set logging file for "json-logger"
	if cfg.Type == jsonfilelog.Name {
		info.LogPath, err = container.GetRootResourcePath(fmt.Sprintf("%s-json.log", container.ID))
		if err != nil {
			return nil, err
		}
	}
	return c(info)
}

// GetProcessLabel returns the process label for the container.
func (container *Container) GetProcessLabel() string {
	// even if we have a process label return "" if we are running
	// in privileged mode
	if container.HostConfig.Privileged {
		return ""
	}
	return container.ProcessLabel
}

// GetMountLabel returns the mounting label for the container.
// This label is empty if the container is privileged.
func (container *Container) GetMountLabel() string {
	return container.MountLabel
}

// GetExecIDs returns the list of exec commands running on the container.
func (container *Container) GetExecIDs() []string {
	return container.ExecCommands.List()
}

// Attach connects to the container's stdio to the client streams
func (container *Container) Attach(cfg *stream.AttachConfig) chan error {
	ctx := container.InitAttachContext()

	cfg.TTY = container.Config.Tty
	if !container.Config.OpenStdin {
		cfg.Stdin = nil
	}
	cfg.CloseStdin = cfg.Stdin != nil && container.Config.StdinOnce

	return container.StreamConfig.Attach(ctx, cfg)
}

// ShouldRestart decides whether the daemon should restart the container or not.
// This is based on the container's restart policy.
func (container *Container) ShouldRestart() bool {
	shouldRestart, _, _ := container.RestartManager().ShouldRestart(uint32(container.ExitCode()), container.HasBeenManuallyStopped, container.FinishedAt.Sub(container.StartedAt))
	return shouldRestart
}

// AddMountPointWithVolume adds a new mount point configured with a volume to the container.
func (container *Container) AddMountPointWithVolume(destination string, vol volume.Volume, rw bool) {
	container.MountPoints[destination] = &volume.MountPoint{
		Type:        mounttypes.TypeVolume,
		Name:        vol.Name(),
		Driver:      vol.DriverName(),
		Destination: destination,
		RW:          rw,
		Volume:      vol,
		CopyData:    volume.DefaultCopyMode,
	}
}

// UnmountVolumes unmounts all volumes
func (container *Container) UnmountVolumes(volumeEventLog func(name, action string, attributes map[string]string)) error {
	var errors []string
	for _, volumeMount := range container.MountPoints {
		// Check if the mountpoint has an ID, this is currently the best way to tell if it's actually mounted
		// TODO(cpuguyh83): there should be a better way to handle this
		if volumeMount.Volume != nil && volumeMount.ID != "" {
			if err := volumeMount.Volume.Unmount(volumeMount.ID); err != nil {
				errors = append(errors, err.Error())
				continue
			}
			volumeMount.ID = ""

			attributes := map[string]string{
				"driver":    volumeMount.Volume.DriverName(),
				"container": container.ID,
			}
			volumeEventLog(volumeMount.Volume.Name(), "unmount", attributes)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("error while unmounting volumes for container %s: %s", container.ID, strings.Join(errors, "; "))
	}
	return nil
}

// IsDestinationMounted checks whether a path is mounted on the container or not.
func (container *Container) IsDestinationMounted(destination string) bool {
	return container.MountPoints[destination] != nil
}

// StopSignal returns the signal used to stop the container.
func (container *Container) StopSignal() int {
	var stopSignal syscall.Signal
	if container.Config.StopSignal != "" {
		stopSignal, _ = signal.ParseSignal(container.Config.StopSignal)
	}

	if int(stopSignal) == 0 {
		stopSignal, _ = signal.ParseSignal(signal.DefaultStopSignal)
	}
	return int(stopSignal)
}

// StopTimeout returns the timeout (in seconds) used to stop the container.
func (container *Container) StopTimeout() int {
	if container.Config.StopTimeout != nil {
		return *container.Config.StopTimeout
	}
	return DefaultStopTimeout
}

// InitDNSHostConfig ensures that the dns fields are never nil.
// New containers don't ever have those fields nil,
// but pre created containers can still have those nil values.
// The non-recommended host configuration in the start api can
// make these fields nil again, this corrects that issue until
// we remove that behavior for good.
// See https://github.com/docker/docker/pull/17779
// for a more detailed explanation on why we don't want that.
func (container *Container) InitDNSHostConfig() {
	container.Lock()
	defer container.Unlock()
	if container.HostConfig.DNS == nil {
		container.HostConfig.DNS = make([]string, 0)
	}

	if container.HostConfig.DNSSearch == nil {
		container.HostConfig.DNSSearch = make([]string, 0)
	}

	if container.HostConfig.DNSOptions == nil {
		container.HostConfig.DNSOptions = make([]string, 0)
	}
}

// GetEndpointInNetwork returns the container's endpoint to the provided network.
func (container *Container) GetEndpointInNetwork(n libnetwork.Network) (libnetwork.Endpoint, error) {
	endpointName := strings.TrimPrefix(container.Name, "/")
	return n.EndpointByName(endpointName)
}

func (container *Container) buildPortMapInfo(ep libnetwork.Endpoint) error {
	if ep == nil {
		return errInvalidEndpoint
	}

	networkSettings := container.NetworkSettings
	if networkSettings == nil {
		return errInvalidNetwork
	}

	if len(networkSettings.Ports) == 0 {
		pm, err := getEndpointPortMapInfo(ep)
		if err != nil {
			return err
		}
		networkSettings.Ports = pm
	}
	return nil
}

func getEndpointPortMapInfo(ep libnetwork.Endpoint) (nat.PortMap, error) {
	pm := nat.PortMap{}
	driverInfo, err := ep.DriverInfo()
	if err != nil {
		return pm, err
	}

	if driverInfo == nil {
		// It is not an error for epInfo to be nil
		return pm, nil
	}

	if expData, ok := driverInfo[netlabel.ExposedPorts]; ok {
		if exposedPorts, ok := expData.([]types.TransportPort); ok {
			for _, tp := range exposedPorts {
				natPort, err := nat.NewPort(tp.Proto.String(), strconv.Itoa(int(tp.Port)))
				if err != nil {
					return pm, fmt.Errorf("Error parsing Port value(%v):%v", tp.Port, err)
				}
				pm[natPort] = nil
			}
		}
	}

	mapData, ok := driverInfo[netlabel.PortMap]
	if !ok {
		return pm, nil
	}

	if portMapping, ok := mapData.([]types.PortBinding); ok {
		for _, pp := range portMapping {
			natPort, err := nat.NewPort(pp.Proto.String(), strconv.Itoa(int(pp.Port)))
			if err != nil {
				return pm, err
			}
			natBndg := nat.PortBinding{HostIP: pp.HostIP.String(), HostPort: strconv.Itoa(int(pp.HostPort))}
			pm[natPort] = append(pm[natPort], natBndg)
		}
	}

	return pm, nil
}

// GetSandboxPortMapInfo retrieves the current port-mapping programmed for the given sandbox
func GetSandboxPortMapInfo(sb libnetwork.Sandbox) nat.PortMap {
	pm := nat.PortMap{}
	if sb == nil {
		return pm
	}

	for _, ep := range sb.Endpoints() {
		pm, _ = getEndpointPortMapInfo(ep)
		if len(pm) > 0 {
			break
		}
	}
	return pm
}

// BuildEndpointInfo sets endpoint-related fields on container.NetworkSettings based on the provided network and endpoint.
func (container *Container) BuildEndpointInfo(n libnetwork.Network, ep libnetwork.Endpoint) error {
	if ep == nil {
		return errInvalidEndpoint
	}

	networkSettings := container.NetworkSettings
	if networkSettings == nil {
		return errInvalidNetwork
	}

	epInfo := ep.Info()
	if epInfo == nil {
		// It is not an error to get an empty endpoint info
		return nil
	}

	if _, ok := networkSettings.Networks[n.Name()]; !ok {
		networkSettings.Networks[n.Name()] = &network.EndpointSettings{
			EndpointSettings: &networktypes.EndpointSettings{},
		}
	}
	networkSettings.Networks[n.Name()].NetworkID = n.ID()
	networkSettings.Networks[n.Name()].EndpointID = ep.ID()

	iface := epInfo.Iface()
	if iface == nil {
		return nil
	}

	if iface.MacAddress() != nil {
		networkSettings.Networks[n.Name()].MacAddress = iface.MacAddress().String()
	}

	if iface.Address() != nil {
		ones, _ := iface.Address().Mask.Size()
		networkSettings.Networks[n.Name()].IPAddress = iface.Address().IP.String()
		networkSettings.Networks[n.Name()].IPPrefixLen = ones
	}

	if iface.AddressIPv6() != nil && iface.AddressIPv6().IP.To16() != nil {
		onesv6, _ := iface.AddressIPv6().Mask.Size()
		networkSettings.Networks[n.Name()].GlobalIPv6Address = iface.AddressIPv6().IP.String()
		networkSettings.Networks[n.Name()].GlobalIPv6PrefixLen = onesv6
	}

	return nil
}

// UpdateJoinInfo updates network settings when container joins network n with endpoint ep.
func (container *Container) UpdateJoinInfo(n libnetwork.Network, ep libnetwork.Endpoint) error {
	if err := container.buildPortMapInfo(ep); err != nil {
		return err
	}

	epInfo := ep.Info()
	if epInfo == nil {
		// It is not an error to get an empty endpoint info
		return nil
	}
	if epInfo.Gateway() != nil {
		container.NetworkSettings.Networks[n.Name()].Gateway = epInfo.Gateway().String()
	}
	if epInfo.GatewayIPv6().To16() != nil {
		container.NetworkSettings.Networks[n.Name()].IPv6Gateway = epInfo.GatewayIPv6().String()
	}

	return nil
}

// UpdateSandboxNetworkSettings updates the sandbox ID and Key.
func (container *Container) UpdateSandboxNetworkSettings(sb libnetwork.Sandbox) error {
	container.NetworkSettings.SandboxID = sb.ID()
	container.NetworkSettings.SandboxKey = sb.Key()
	return nil
}

// BuildJoinOptions builds endpoint Join options from a given network.
func (container *Container) BuildJoinOptions(n libnetwork.Network) ([]libnetwork.EndpointOption, error) {
	var joinOptions []libnetwork.EndpointOption
	if epConfig, ok := container.NetworkSettings.Networks[n.Name()]; ok {
		for _, str := range epConfig.Links {
			name, alias, err := opts.ParseLink(str)
			if err != nil {
				return nil, err
			}
			joinOptions = append(joinOptions, libnetwork.CreateOptionAlias(name, alias))
		}
	}
	return joinOptions, nil
}

// BuildCreateEndpointOptions builds endpoint options from a given network.
func (container *Container) BuildCreateEndpointOptions(n libnetwork.Network, epConfig *networktypes.EndpointSettings, sb libnetwork.Sandbox, daemonDNS []string) ([]libnetwork.EndpointOption, error) {
	var (
		bindings      = make(nat.PortMap)
		pbList        []types.PortBinding
		exposeList    []types.TransportPort
		createOptions []libnetwork.EndpointOption
	)

	defaultNetName := runconfig.DefaultDaemonNetworkMode().NetworkName()

	if (!container.EnableServiceDiscoveryOnDefaultNetwork() && n.Name() == defaultNetName) ||
		container.NetworkSettings.IsAnonymousEndpoint {
		createOptions = append(createOptions, libnetwork.CreateOptionAnonymous())
	}

	if epConfig != nil {
		ipam := epConfig.IPAMConfig
		if ipam != nil && (ipam.IPv4Address != "" || ipam.IPv6Address != "" || len(ipam.LinkLocalIPs) > 0) {
			var ipList []net.IP
			for _, ips := range ipam.LinkLocalIPs {
				if ip := net.ParseIP(ips); ip != nil {
					ipList = append(ipList, ip)
				}
			}
			createOptions = append(createOptions,
				libnetwork.CreateOptionIpam(net.ParseIP(ipam.IPv4Address), net.ParseIP(ipam.IPv6Address), ipList, nil))
		}

		for _, alias := range epConfig.Aliases {
			createOptions = append(createOptions, libnetwork.CreateOptionMyAlias(alias))
		}
	}

	if container.NetworkSettings.Service != nil {
		svcCfg := container.NetworkSettings.Service

		var vip string
		if svcCfg.VirtualAddresses[n.ID()] != nil {
			vip = svcCfg.VirtualAddresses[n.ID()].IPv4
		}

		var portConfigs []*libnetwork.PortConfig
		for _, portConfig := range svcCfg.ExposedPorts {
			portConfigs = append(portConfigs, &libnetwork.PortConfig{
				Name:          portConfig.Name,
				Protocol:      libnetwork.PortConfig_Protocol(portConfig.Protocol),
				TargetPort:    portConfig.TargetPort,
				PublishedPort: portConfig.PublishedPort,
			})
		}

		createOptions = append(createOptions, libnetwork.CreateOptionService(svcCfg.Name, svcCfg.ID, net.ParseIP(vip), portConfigs, svcCfg.Aliases[n.ID()]))
	}

	if !containertypes.NetworkMode(n.Name()).IsUserDefined() {
		createOptions = append(createOptions, libnetwork.CreateOptionDisableResolution())
	}

	// configs that are applicable only for the endpoint in the network
	// to which container was connected to on docker run.
	// Ideally all these network-specific endpoint configurations must be moved under
	// container.NetworkSettings.Networks[n.Name()]
	if n.Name() == container.HostConfig.NetworkMode.NetworkName() ||
		(n.Name() == defaultNetName && container.HostConfig.NetworkMode.IsDefault()) {
		if container.Config.MacAddress != "" {
			mac, err := net.ParseMAC(container.Config.MacAddress)
			if err != nil {
				return nil, err
			}

			genericOption := options.Generic{
				netlabel.MacAddress: mac,
			}

			createOptions = append(createOptions, libnetwork.EndpointOptionGeneric(genericOption))
		}
	}

	// Port-mapping rules belong to the container & applicable only to non-internal networks
	portmaps := GetSandboxPortMapInfo(sb)
	if n.Info().Internal() || len(portmaps) > 0 {
		return createOptions, nil
	}

	if container.HostConfig.PortBindings != nil {
		for p, b := range container.HostConfig.PortBindings {
			bindings[p] = []nat.PortBinding{}
			for _, bb := range b {
				bindings[p] = append(bindings[p], nat.PortBinding{
					HostIP:   bb.HostIP,
					HostPort: bb.HostPort,
				})
			}
		}
	}

	portSpecs := container.Config.ExposedPorts
	ports := make([]nat.Port, len(portSpecs))
	var i int
	for p := range portSpecs {
		ports[i] = p
		i++
	}
	nat.SortPortMap(ports, bindings)
	for _, port := range ports {
		expose := types.TransportPort{}
		expose.Proto = types.ParseProtocol(port.Proto())
		expose.Port = uint16(port.Int())
		exposeList = append(exposeList, expose)

		pb := types.PortBinding{Port: expose.Port, Proto: expose.Proto}
		binding := bindings[port]
		for i := 0; i < len(binding); i++ {
			pbCopy := pb.GetCopy()
			newP, err := nat.NewPort(nat.SplitProtoPort(binding[i].HostPort))
			var portStart, portEnd int
			if err == nil {
				portStart, portEnd, err = newP.Range()
			}
			if err != nil {
				return nil, fmt.Errorf("Error parsing HostPort value(%s):%v", binding[i].HostPort, err)
			}
			pbCopy.HostPort = uint16(portStart)
			pbCopy.HostPortEnd = uint16(portEnd)
			pbCopy.HostIP = net.ParseIP(binding[i].HostIP)
			pbList = append(pbList, pbCopy)
		}

		if container.HostConfig.PublishAllPorts && len(binding) == 0 {
			pbList = append(pbList, pb)
		}
	}

	var dns []string

	if len(container.HostConfig.DNS) > 0 {
		dns = container.HostConfig.DNS
	} else if len(daemonDNS) > 0 {
		dns = daemonDNS
	}

	if len(dns) > 0 {
		createOptions = append(createOptions,
			libnetwork.CreateOptionDNS(dns))
	}

	createOptions = append(createOptions,
		libnetwork.CreateOptionPortMapping(pbList),
		libnetwork.CreateOptionExposedPorts(exposeList))

	return createOptions, nil
}

// UpdateMonitor updates monitor configure for running container
func (container *Container) UpdateMonitor(restartPolicy containertypes.RestartPolicy) {
	type policySetter interface {
		SetPolicy(containertypes.RestartPolicy)
	}

	if rm, ok := container.RestartManager().(policySetter); ok {
		rm.SetPolicy(restartPolicy)
	}
}

// FullHostname returns hostname and optional domain appended to it.
func (container *Container) FullHostname() string {
	fullHostname := container.Config.Hostname
	if container.Config.Domainname != "" {
		fullHostname = fmt.Sprintf("%s.%s", fullHostname, container.Config.Domainname)
	}
	return fullHostname
}

// RestartManager returns the current restartmanager instance connected to container.
func (container *Container) RestartManager() restartmanager.RestartManager {
	if container.restartManager == nil {
		container.restartManager = restartmanager.New(container.HostConfig.RestartPolicy, container.RestartCount)
	}
	return container.restartManager
}

// ResetRestartManager initializes new restartmanager based on container config
func (container *Container) ResetRestartManager(resetCount bool) {
	if container.restartManager != nil {
		container.restartManager.Cancel()
	}
	if resetCount {
		container.RestartCount = 0
	}
	container.restartManager = nil
}

type attachContext struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
}

// InitAttachContext initializes or returns existing context for attach calls to
// track container liveness.
func (container *Container) InitAttachContext() context.Context {
	container.attachContext.mu.Lock()
	defer container.attachContext.mu.Unlock()
	if container.attachContext.ctx == nil {
		container.attachContext.ctx, container.attachContext.cancel = context.WithCancel(context.Background())
	}
	return container.attachContext.ctx
}

// CancelAttachContext cancels attach context. All attach calls should detach
// after this call.
func (container *Container) CancelAttachContext() {
	container.attachContext.mu.Lock()
	if container.attachContext.ctx != nil {
		container.attachContext.cancel()
		container.attachContext.ctx = nil
	}
	container.attachContext.mu.Unlock()
}

func (container *Container) startLogging() error {
	if container.HostConfig.LogConfig.Type == "none" {
		return nil // do not start logging routines
	}

	l, err := container.StartLogger()
	if err != nil {
		return fmt.Errorf("failed to initialize logging driver: %v", err)
	}

	copier := logger.NewCopier(map[string]io.Reader{"stdout": container.StdoutPipe(), "stderr": container.StderrPipe()}, l)
	container.LogCopier = copier
	copier.Run()
	container.LogDriver = l

	// set LogPath field only for json-file logdriver
	if jl, ok := l.(*jsonfilelog.JSONFileLogger); ok {
		container.LogPath = jl.LogPath()
	}

	return nil
}

// StdinPipe gets the stdin stream of the container
func (container *Container) StdinPipe() io.WriteCloser {
	return container.StreamConfig.StdinPipe()
}

// StdoutPipe gets the stdout stream of the container
func (container *Container) StdoutPipe() io.ReadCloser {
	return container.StreamConfig.StdoutPipe()
}

// StderrPipe gets the stderr stream of the container
func (container *Container) StderrPipe() io.ReadCloser {
	return container.StreamConfig.StderrPipe()
}

// CloseStreams closes the container's stdio streams
func (container *Container) CloseStreams() error {
	return container.StreamConfig.CloseStreams()
}

// InitializeStdio is called by libcontainerd to connect the stdio.
func (container *Container) InitializeStdio(iop libcontainerd.IOPipe) error {
	if err := container.startLogging(); err != nil {
		container.Reset(false)
		return err
	}

	container.StreamConfig.CopyToPipe(iop)

	if container.StreamConfig.Stdin() == nil && !container.Config.Tty {
		if iop.Stdin != nil {
			if err := iop.Stdin.Close(); err != nil {
				logrus.Warnf("error closing stdin: %+v", err)
			}
		}
	}

	return nil
}
