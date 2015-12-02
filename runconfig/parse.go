package runconfig

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/pkg/units"
	"github.com/docker/docker/volume"
)

var (
	// ErrConflictContainerNetworkAndLinks conflict between --net=container and links
	ErrConflictContainerNetworkAndLinks = fmt.Errorf("Conflicting options: --net=container can't be used with links. This would result in undefined behavior")
	// ErrConflictUserDefinedNetworkAndLinks conflict between --net=<NETWORK> and links
	ErrConflictUserDefinedNetworkAndLinks = fmt.Errorf("Conflicting options: --net=<NETWORK> can't be used with links. This would result in undefined behavior")
	// ErrConflictSharedNetwork conflict between private and other networks
	ErrConflictSharedNetwork = fmt.Errorf("Container sharing network namespace with another container or host cannot be connected to any other network")
	// ErrConflictHostNetwork conflict from being disconnected from host network or connected to host network.
	ErrConflictHostNetwork = fmt.Errorf("Container cannot be disconnected from host network or connected to host network")
	// ErrConflictNoNetwork conflict between private and other networks
	ErrConflictNoNetwork = fmt.Errorf("Container cannot be connected to multiple networks with one of the networks in --none mode")
	// ErrConflictNetworkAndDNS conflict between --dns and the network mode
	ErrConflictNetworkAndDNS = fmt.Errorf("Conflicting options: --dns and the network mode (--net)")
	// ErrConflictNetworkHostname conflict between the hostname and the network mode
	ErrConflictNetworkHostname = fmt.Errorf("Conflicting options: -h and the network mode (--net)")
	// ErrConflictHostNetworkAndLinks conflict between --net=host and links
	ErrConflictHostNetworkAndLinks = fmt.Errorf("Conflicting options: --net=host can't be used with links. This would result in undefined behavior")
	// ErrConflictContainerNetworkAndMac conflict between the mac address and the network mode
	ErrConflictContainerNetworkAndMac = fmt.Errorf("Conflicting options: --mac-address and the network mode (--net)")
	// ErrConflictNetworkHosts conflict between add-host and the network mode
	ErrConflictNetworkHosts = fmt.Errorf("Conflicting options: --add-host and the network mode (--net)")
	// ErrConflictNetworkPublishPorts conflict between the pulbish options and the network mode
	ErrConflictNetworkPublishPorts = fmt.Errorf("Conflicting options: -p, -P, --publish-all, --publish and the network mode (--net)")
	// ErrConflictNetworkExposePorts conflict between the expose option and the network mode
	ErrConflictNetworkExposePorts = fmt.Errorf("Conflicting options: --expose and the network mode (--net)")
)

// Parse parses the specified args for the specified command and generates a Config,
// a HostConfig and returns them with the specified command.
// If the specified args are not valid, it will return an error.
func Parse(cmd *flag.FlagSet, args []string) (*Config, *HostConfig, *flag.FlagSet, error) {
	var (
		// FIXME: use utils.ListOpts for attach and volumes?
		flAttach            = opts.NewListOpts(opts.ValidateAttach)
		flVolumes           = opts.NewListOpts(nil)
		flBlkioWeightDevice = opts.NewWeightdeviceOpt(opts.ValidateWeightDevice)
		flLinks             = opts.NewListOpts(opts.ValidateLink)
		flEnv               = opts.NewListOpts(opts.ValidateEnv)
		flLabels            = opts.NewListOpts(opts.ValidateEnv)
		flDevices           = opts.NewListOpts(opts.ValidateDevice)

		flUlimits = opts.NewUlimitOpt(nil)

		flPublish           = opts.NewListOpts(nil)
		flExpose            = opts.NewListOpts(nil)
		flDNS               = opts.NewListOpts(opts.ValidateIPAddress)
		flDNSSearch         = opts.NewListOpts(opts.ValidateDNSSearch)
		flDNSOptions        = opts.NewListOpts(nil)
		flExtraHosts        = opts.NewListOpts(opts.ValidateExtraHost)
		flVolumesFrom       = opts.NewListOpts(nil)
		flEnvFile           = opts.NewListOpts(nil)
		flCapAdd            = opts.NewListOpts(nil)
		flCapDrop           = opts.NewListOpts(nil)
		flGroupAdd          = opts.NewListOpts(nil)
		flSecurityOpt       = opts.NewListOpts(nil)
		flLabelsFile        = opts.NewListOpts(nil)
		flLoggingOpts       = opts.NewListOpts(nil)
		flPrivileged        = cmd.Bool([]string{"-privileged"}, false, "Give extended privileges to this container")
		flPidMode           = cmd.String([]string{"-pid"}, "", "PID namespace to use")
		flUTSMode           = cmd.String([]string{"-uts"}, "", "UTS namespace to use")
		flPublishAll        = cmd.Bool([]string{"P", "-publish-all"}, false, "Publish all exposed ports to random ports")
		flStdin             = cmd.Bool([]string{"i", "-interactive"}, false, "Keep STDIN open even if not attached")
		flTty               = cmd.Bool([]string{"t", "-tty"}, false, "Allocate a pseudo-TTY")
		flOomKillDisable    = cmd.Bool([]string{"-oom-kill-disable"}, false, "Disable OOM Killer")
		flContainerIDFile   = cmd.String([]string{"-cidfile"}, "", "Write the container ID to the file")
		flEntrypoint        = cmd.String([]string{"-entrypoint"}, "", "Overwrite the default ENTRYPOINT of the image")
		flHostname          = cmd.String([]string{"h", "-hostname"}, "", "Container host name")
		flMemoryString      = cmd.String([]string{"m", "-memory"}, "", "Memory limit")
		flMemoryReservation = cmd.String([]string{"-memory-reservation"}, "", "Memory soft limit")
		flMemorySwap        = cmd.String([]string{"-memory-swap"}, "", "Total memory (memory + swap), '-1' to disable swap")
		flKernelMemory      = cmd.String([]string{"-kernel-memory"}, "", "Kernel memory limit")
		flUser              = cmd.String([]string{"u", "-user"}, "", "Username or UID (format: <name|uid>[:<group|gid>])")
		flWorkingDir        = cmd.String([]string{"w", "-workdir"}, "", "Working directory inside the container")
		flCPUShares         = cmd.Int64([]string{"#c", "-cpu-shares"}, 0, "CPU shares (relative weight)")
		flCPUPeriod         = cmd.Int64([]string{"-cpu-period"}, 0, "Limit CPU CFS (Completely Fair Scheduler) period")
		flCPUQuota          = cmd.Int64([]string{"-cpu-quota"}, 0, "Limit CPU CFS (Completely Fair Scheduler) quota")
		flCpusetCpus        = cmd.String([]string{"-cpuset-cpus"}, "", "CPUs in which to allow execution (0-3, 0,1)")
		flCpusetMems        = cmd.String([]string{"-cpuset-mems"}, "", "MEMs in which to allow execution (0-3, 0,1)")
		flBlkioWeight       = cmd.Uint16([]string{"-blkio-weight"}, 0, "Block IO (relative weight), between 10 and 1000")
		flSwappiness        = cmd.Int64([]string{"-memory-swappiness"}, -1, "Tuning container memory swappiness (0 to 100)")
		flNetMode           = cmd.String([]string{"-net"}, "default", "Set the Network for the container")
		flMacAddress        = cmd.String([]string{"-mac-address"}, "", "Container MAC address (e.g. 92:d0:c6:0a:29:33)")
		flIpcMode           = cmd.String([]string{"-ipc"}, "", "IPC namespace to use")
		flRestartPolicy     = cmd.String([]string{"-restart"}, "no", "Restart policy to apply when a container exits")
		flReadonlyRootfs    = cmd.Bool([]string{"-read-only"}, false, "Mount the container's root filesystem as read only")
		flLoggingDriver     = cmd.String([]string{"-log-driver"}, "", "Logging driver for container")
		flCgroupParent      = cmd.String([]string{"-cgroup-parent"}, "", "Optional parent cgroup for the container")
		flVolumeDriver      = cmd.String([]string{"-volume-driver"}, "", "Optional volume driver for the container")
		flStopSignal        = cmd.String([]string{"-stop-signal"}, signal.DefaultStopSignal, fmt.Sprintf("Signal to stop a container, %v by default", signal.DefaultStopSignal))
		flIsolation         = cmd.String([]string{"-isolation"}, "", "Container isolation level")
		flShmSize           = cmd.String([]string{"-shm-size"}, "", "Size of /dev/shm, default value is 64MB")
	)

	cmd.Var(&flAttach, []string{"a", "-attach"}, "Attach to STDIN, STDOUT or STDERR")
	cmd.Var(&flBlkioWeightDevice, []string{"-blkio-weight-device"}, "Block IO weight (relative device weight)")
	cmd.Var(&flVolumes, []string{"v", "-volume"}, "Bind mount a volume")
	cmd.Var(&flLinks, []string{"-link"}, "Add link to another container")
	cmd.Var(&flDevices, []string{"-device"}, "Add a host device to the container")
	cmd.Var(&flLabels, []string{"l", "-label"}, "Set meta data on a container")
	cmd.Var(&flLabelsFile, []string{"-label-file"}, "Read in a line delimited file of labels")
	cmd.Var(&flEnv, []string{"e", "-env"}, "Set environment variables")
	cmd.Var(&flEnvFile, []string{"-env-file"}, "Read in a file of environment variables")
	cmd.Var(&flPublish, []string{"p", "-publish"}, "Publish a container's port(s) to the host")
	cmd.Var(&flExpose, []string{"-expose"}, "Expose a port or a range of ports")
	cmd.Var(&flDNS, []string{"-dns"}, "Set custom DNS servers")
	cmd.Var(&flDNSSearch, []string{"-dns-search"}, "Set custom DNS search domains")
	cmd.Var(&flDNSOptions, []string{"-dns-opt"}, "Set DNS options")
	cmd.Var(&flExtraHosts, []string{"-add-host"}, "Add a custom host-to-IP mapping (host:ip)")
	cmd.Var(&flVolumesFrom, []string{"-volumes-from"}, "Mount volumes from the specified container(s)")
	cmd.Var(&flCapAdd, []string{"-cap-add"}, "Add Linux capabilities")
	cmd.Var(&flCapDrop, []string{"-cap-drop"}, "Drop Linux capabilities")
	cmd.Var(&flGroupAdd, []string{"-group-add"}, "Add additional groups to join")
	cmd.Var(&flSecurityOpt, []string{"-security-opt"}, "Security Options")
	cmd.Var(flUlimits, []string{"-ulimit"}, "Ulimit options")
	cmd.Var(&flLoggingOpts, []string{"-log-opt"}, "Log driver options")

	cmd.Require(flag.Min, 1)

	if err := cmd.ParseFlags(args, true); err != nil {
		return nil, nil, cmd, err
	}

	var (
		attachStdin  = flAttach.Get("stdin")
		attachStdout = flAttach.Get("stdout")
		attachStderr = flAttach.Get("stderr")
	)

	// Validate the input mac address
	if *flMacAddress != "" {
		if _, err := opts.ValidateMACAddress(*flMacAddress); err != nil {
			return nil, nil, cmd, fmt.Errorf("%s is not a valid mac address", *flMacAddress)
		}
	}
	if *flStdin {
		attachStdin = true
	}
	// If -a is not set attach to the output stdio
	if flAttach.Len() == 0 {
		attachStdout = true
		attachStderr = true
	}

	var err error

	var flMemory int64
	if *flMemoryString != "" {
		flMemory, err = units.RAMInBytes(*flMemoryString)
		if err != nil {
			return nil, nil, cmd, err
		}
	}

	var MemoryReservation int64
	if *flMemoryReservation != "" {
		MemoryReservation, err = units.RAMInBytes(*flMemoryReservation)
		if err != nil {
			return nil, nil, cmd, err
		}
	}

	var memorySwap int64
	if *flMemorySwap != "" {
		if *flMemorySwap == "-1" {
			memorySwap = -1
		} else {
			memorySwap, err = units.RAMInBytes(*flMemorySwap)
			if err != nil {
				return nil, nil, cmd, err
			}
		}
	}

	var KernelMemory int64
	if *flKernelMemory != "" {
		KernelMemory, err = units.RAMInBytes(*flKernelMemory)
		if err != nil {
			return nil, nil, cmd, err
		}
	}

	swappiness := *flSwappiness
	if swappiness != -1 && (swappiness < 0 || swappiness > 100) {
		return nil, nil, cmd, fmt.Errorf("Invalid value: %d. Valid memory swappiness range is 0-100", swappiness)
	}

	var parsedShm *int64
	if *flShmSize != "" {
		shmSize, err := units.RAMInBytes(*flShmSize)
		if err != nil {
			return nil, nil, cmd, err
		}
		parsedShm = &shmSize
	}

	var binds []string
	// add any bind targets to the list of container volumes
	for bind := range flVolumes.GetMap() {
		if arr := volume.SplitN(bind, 2); len(arr) > 1 {
			// after creating the bind mount we want to delete it from the flVolumes values because
			// we do not want bind mounts being committed to image configs
			binds = append(binds, bind)
			flVolumes.Delete(bind)
		}
	}

	var (
		parsedArgs = cmd.Args()
		runCmd     *stringutils.StrSlice
		entrypoint *stringutils.StrSlice
		image      = cmd.Arg(0)
	)
	if len(parsedArgs) > 1 {
		runCmd = stringutils.NewStrSlice(parsedArgs[1:]...)
	}
	if *flEntrypoint != "" {
		entrypoint = stringutils.NewStrSlice(*flEntrypoint)
	}

	var (
		domainname string
		hostname   = *flHostname
		parts      = strings.SplitN(hostname, ".", 2)
	)
	if len(parts) > 1 {
		hostname = parts[0]
		domainname = parts[1]
	}

	ports, portBindings, err := nat.ParsePortSpecs(flPublish.GetAll())
	if err != nil {
		return nil, nil, cmd, err
	}

	// Merge in exposed ports to the map of published ports
	for _, e := range flExpose.GetAll() {
		if strings.Contains(e, ":") {
			return nil, nil, cmd, fmt.Errorf("Invalid port format for --expose: %s", e)
		}
		//support two formats for expose, original format <portnum>/[<proto>] or <startport-endport>/[<proto>]
		proto, port := nat.SplitProtoPort(e)
		//parse the start and end port and create a sequence of ports to expose
		//if expose a port, the start and end port are the same
		start, end, err := parsers.ParsePortRange(port)
		if err != nil {
			return nil, nil, cmd, fmt.Errorf("Invalid range format for --expose: %s, error: %s", e, err)
		}
		for i := start; i <= end; i++ {
			p, err := nat.NewPort(proto, strconv.FormatUint(i, 10))
			if err != nil {
				return nil, nil, cmd, err
			}
			if _, exists := ports[p]; !exists {
				ports[p] = struct{}{}
			}
		}
	}

	// parse device mappings
	deviceMappings := []DeviceMapping{}
	for _, device := range flDevices.GetAll() {
		deviceMapping, err := ParseDevice(device)
		if err != nil {
			return nil, nil, cmd, err
		}
		deviceMappings = append(deviceMappings, deviceMapping)
	}

	// collect all the environment variables for the container
	envVariables, err := readKVStrings(flEnvFile.GetAll(), flEnv.GetAll())
	if err != nil {
		return nil, nil, cmd, err
	}

	// collect all the labels for the container
	labels, err := readKVStrings(flLabelsFile.GetAll(), flLabels.GetAll())
	if err != nil {
		return nil, nil, cmd, err
	}

	ipcMode := IpcMode(*flIpcMode)
	if !ipcMode.Valid() {
		return nil, nil, cmd, fmt.Errorf("--ipc: invalid IPC mode")
	}

	pidMode := PidMode(*flPidMode)
	if !pidMode.Valid() {
		return nil, nil, cmd, fmt.Errorf("--pid: invalid PID mode")
	}

	utsMode := UTSMode(*flUTSMode)
	if !utsMode.Valid() {
		return nil, nil, cmd, fmt.Errorf("--uts: invalid UTS mode")
	}

	restartPolicy, err := ParseRestartPolicy(*flRestartPolicy)
	if err != nil {
		return nil, nil, cmd, err
	}

	loggingOpts, err := parseLoggingOpts(*flLoggingDriver, flLoggingOpts.GetAll())
	if err != nil {
		return nil, nil, cmd, err
	}

	resources := Resources{
		CgroupParent:      *flCgroupParent,
		Memory:            flMemory,
		MemoryReservation: MemoryReservation,
		MemorySwap:        memorySwap,
		MemorySwappiness:  flSwappiness,
		KernelMemory:      KernelMemory,
		CPUShares:         *flCPUShares,
		CPUPeriod:         *flCPUPeriod,
		CpusetCpus:        *flCpusetCpus,
		CpusetMems:        *flCpusetMems,
		CPUQuota:          *flCPUQuota,
		BlkioWeight:       *flBlkioWeight,
		BlkioWeightDevice: flBlkioWeightDevice.GetList(),
		Ulimits:           flUlimits.GetList(),
		Devices:           deviceMappings,
	}

	config := &Config{
		Hostname:     hostname,
		Domainname:   domainname,
		ExposedPorts: ports,
		User:         *flUser,
		Tty:          *flTty,
		// TODO: deprecated, it comes from -n, --networking
		// it's still needed internally to set the network to disabled
		// if e.g. bridge is none in daemon opts, and in inspect
		NetworkDisabled: false,
		OpenStdin:       *flStdin,
		AttachStdin:     attachStdin,
		AttachStdout:    attachStdout,
		AttachStderr:    attachStderr,
		Env:             envVariables,
		Cmd:             runCmd,
		Image:           image,
		Volumes:         flVolumes.GetMap(),
		MacAddress:      *flMacAddress,
		Entrypoint:      entrypoint,
		WorkingDir:      *flWorkingDir,
		Labels:          ConvertKVStringsToMap(labels),
		StopSignal:      *flStopSignal,
	}

	hostConfig := &HostConfig{
		Binds:           binds,
		ContainerIDFile: *flContainerIDFile,
		OomKillDisable:  *flOomKillDisable,
		Privileged:      *flPrivileged,
		PortBindings:    portBindings,
		Links:           flLinks.GetAll(),
		PublishAllPorts: *flPublishAll,
		// Make sure the dns fields are never nil.
		// New containers don't ever have those fields nil,
		// but pre created containers can still have those nil values.
		// See https://github.com/docker/docker/pull/17779
		// for a more detailed explanation on why we don't want that.
		DNS:            flDNS.GetAllOrEmpty(),
		DNSSearch:      flDNSSearch.GetAllOrEmpty(),
		DNSOptions:     flDNSOptions.GetAllOrEmpty(),
		ExtraHosts:     flExtraHosts.GetAll(),
		VolumesFrom:    flVolumesFrom.GetAll(),
		NetworkMode:    NetworkMode(*flNetMode),
		IpcMode:        ipcMode,
		PidMode:        pidMode,
		UTSMode:        utsMode,
		CapAdd:         stringutils.NewStrSlice(flCapAdd.GetAll()...),
		CapDrop:        stringutils.NewStrSlice(flCapDrop.GetAll()...),
		GroupAdd:       flGroupAdd.GetAll(),
		RestartPolicy:  restartPolicy,
		SecurityOpt:    flSecurityOpt.GetAll(),
		ReadonlyRootfs: *flReadonlyRootfs,
		LogConfig:      LogConfig{Type: *flLoggingDriver, Config: loggingOpts},
		VolumeDriver:   *flVolumeDriver,
		Isolation:      IsolationLevel(*flIsolation),
		ShmSize:        parsedShm,
		Resources:      resources,
	}

	// When allocating stdin in attached mode, close stdin at client disconnect
	if config.OpenStdin && config.AttachStdin {
		config.StdinOnce = true
	}
	return config, hostConfig, cmd, nil
}

// reads a file of line terminated key=value pairs and override that with override parameter
func readKVStrings(files []string, override []string) ([]string, error) {
	envVariables := []string{}
	for _, ef := range files {
		parsedVars, err := opts.ParseEnvFile(ef)
		if err != nil {
			return nil, err
		}
		envVariables = append(envVariables, parsedVars...)
	}
	// parse the '-e' and '--env' after, to allow override
	envVariables = append(envVariables, override...)

	return envVariables, nil
}

// ConvertKVStringsToMap converts ["key=value"] to {"key":"value"}
func ConvertKVStringsToMap(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		kv := strings.SplitN(value, "=", 2)
		if len(kv) == 1 {
			result[kv[0]] = ""
		} else {
			result[kv[0]] = kv[1]
		}
	}

	return result
}

func parseLoggingOpts(loggingDriver string, loggingOpts []string) (map[string]string, error) {
	loggingOptsMap := ConvertKVStringsToMap(loggingOpts)
	if loggingDriver == "none" && len(loggingOpts) > 0 {
		return map[string]string{}, fmt.Errorf("Invalid logging opts for driver %s", loggingDriver)
	}
	return loggingOptsMap, nil
}

// ParseRestartPolicy returns the parsed policy or an error indicating what is incorrect
func ParseRestartPolicy(policy string) (RestartPolicy, error) {
	p := RestartPolicy{}

	if policy == "" {
		return p, nil
	}

	var (
		parts = strings.Split(policy, ":")
		name  = parts[0]
	)

	p.Name = name
	switch name {
	case "always", "unless-stopped":
		if len(parts) > 1 {
			return p, fmt.Errorf("maximum restart count not valid with restart policy of \"%s\"", name)
		}
	case "no":
		// do nothing
	case "on-failure":
		if len(parts) > 2 {
			return p, fmt.Errorf("restart count format is not valid, usage: 'on-failure:N' or 'on-failure'")
		}
		if len(parts) == 2 {
			count, err := strconv.Atoi(parts[1])
			if err != nil {
				return p, err
			}

			p.MaximumRetryCount = count
		}
	default:
		return p, fmt.Errorf("invalid restart policy %s", name)
	}

	return p, nil
}

func parseKeyValueOpts(opts opts.ListOpts) ([]KeyValuePair, error) {
	out := make([]KeyValuePair, opts.Len())
	for i, o := range opts.GetAll() {
		k, v, err := parsers.ParseKeyValueOpt(o)
		if err != nil {
			return nil, err
		}
		out[i] = KeyValuePair{Key: k, Value: v}
	}
	return out, nil
}

// ParseDevice parses a device mapping string to a DeviceMapping struct
func ParseDevice(device string) (DeviceMapping, error) {
	src := ""
	dst := ""
	permissions := "rwm"
	arr := strings.Split(device, ":")
	switch len(arr) {
	case 3:
		permissions = arr[2]
		fallthrough
	case 2:
		if opts.ValidDeviceMode(arr[1]) {
			permissions = arr[1]
		} else {
			dst = arr[1]
		}
		fallthrough
	case 1:
		src = arr[0]
	default:
		return DeviceMapping{}, fmt.Errorf("Invalid device specification: %s", device)
	}

	if dst == "" {
		dst = src
	}

	deviceMapping := DeviceMapping{
		PathOnHost:        src,
		PathInContainer:   dst,
		CgroupPermissions: permissions,
	}
	return deviceMapping, nil
}
