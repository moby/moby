package opts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/engine-api/types/container"
	networktypes "github.com/docker/engine-api/types/network"
	"github.com/docker/engine-api/types/strslice"
	"github.com/docker/go-connections/nat"
	units "github.com/docker/go-units"
	"github.com/spf13/pflag"
)

// ContainerOptions is a data object with all the options for creating a container
// TODO: remove fl prefix
type ContainerOptions struct {
	flAttach            opts.ListOpts
	flVolumes           opts.ListOpts
	flTmpfs             opts.ListOpts
	flBlkioWeightDevice WeightdeviceOpt
	flDeviceReadBps     ThrottledeviceOpt
	flDeviceWriteBps    ThrottledeviceOpt
	flLinks             opts.ListOpts
	flAliases           opts.ListOpts
	flDeviceReadIOps    ThrottledeviceOpt
	flDeviceWriteIOps   ThrottledeviceOpt
	flEnv               opts.ListOpts
	flLabels            opts.ListOpts
	flDevices           opts.ListOpts
	flUlimits           *UlimitOpt
	flSysctls           *opts.MapOpts
	flPublish           opts.ListOpts
	flExpose            opts.ListOpts
	flDNS               opts.ListOpts
	flDNSSearch         opts.ListOpts
	flDNSOptions        opts.ListOpts
	flExtraHosts        opts.ListOpts
	flVolumesFrom       opts.ListOpts
	flEnvFile           opts.ListOpts
	flCapAdd            opts.ListOpts
	flCapDrop           opts.ListOpts
	flGroupAdd          opts.ListOpts
	flSecurityOpt       opts.ListOpts
	flStorageOpt        opts.ListOpts
	flLabelsFile        opts.ListOpts
	flLoggingOpts       opts.ListOpts
	flPrivileged        *bool
	flPidMode           *string
	flUTSMode           *string
	flUsernsMode        *string
	flPublishAll        *bool
	flStdin             *bool
	flTty               *bool
	flOomKillDisable    *bool
	flOomScoreAdj       *int
	flContainerIDFile   *string
	flEntrypoint        *string
	flHostname          *string
	flMemoryString      *string
	flMemoryReservation *string
	flMemorySwap        *string
	flKernelMemory      *string
	flUser              *string
	flWorkingDir        *string
	flCPUShares         *int64
	flCPUPercent        *int64
	flCPUPeriod         *int64
	flCPUQuota          *int64
	flCpusetCpus        *string
	flCpusetMems        *string
	flBlkioWeight       *uint16
	flIOMaxBandwidth    *string
	flIOMaxIOps         *uint64
	flSwappiness        *int64
	flNetMode           *string
	flMacAddress        *string
	flIPv4Address       *string
	flIPv6Address       *string
	flIpcMode           *string
	flPidsLimit         *int64
	flRestartPolicy     *string
	flReadonlyRootfs    *bool
	flLoggingDriver     *string
	flCgroupParent      *string
	flVolumeDriver      *string
	flStopSignal        *string
	flIsolation         *string
	flShmSize           *string
	flNoHealthcheck     *bool
	flHealthCmd         *string
	flHealthInterval    *time.Duration
	flHealthTimeout     *time.Duration
	flHealthRetries     *int

	Image string
	Args  []string
}

// AddFlags adds all command line flags that will be used by Parse to the FlagSet
func AddFlags(flags *pflag.FlagSet) *ContainerOptions {
	copts := &ContainerOptions{
		flAttach:            opts.NewListOpts(ValidateAttach),
		flVolumes:           opts.NewListOpts(nil),
		flTmpfs:             opts.NewListOpts(nil),
		flBlkioWeightDevice: NewWeightdeviceOpt(ValidateWeightDevice),
		flDeviceReadBps:     NewThrottledeviceOpt(ValidateThrottleBpsDevice),
		flDeviceWriteBps:    NewThrottledeviceOpt(ValidateThrottleBpsDevice),
		flLinks:             opts.NewListOpts(ValidateLink),
		flAliases:           opts.NewListOpts(nil),
		flDeviceReadIOps:    NewThrottledeviceOpt(ValidateThrottleIOpsDevice),
		flDeviceWriteIOps:   NewThrottledeviceOpt(ValidateThrottleIOpsDevice),
		flEnv:               opts.NewListOpts(ValidateEnv),
		flLabels:            opts.NewListOpts(ValidateEnv),
		flDevices:           opts.NewListOpts(ValidateDevice),

		flUlimits: NewUlimitOpt(nil),
		flSysctls: opts.NewMapOpts(nil, opts.ValidateSysctl),

		flPublish:     opts.NewListOpts(nil),
		flExpose:      opts.NewListOpts(nil),
		flDNS:         opts.NewListOpts(opts.ValidateIPAddress),
		flDNSSearch:   opts.NewListOpts(opts.ValidateDNSSearch),
		flDNSOptions:  opts.NewListOpts(nil),
		flExtraHosts:  opts.NewListOpts(ValidateExtraHost),
		flVolumesFrom: opts.NewListOpts(nil),
		flEnvFile:     opts.NewListOpts(nil),
		flCapAdd:      opts.NewListOpts(nil),
		flCapDrop:     opts.NewListOpts(nil),
		flGroupAdd:    opts.NewListOpts(nil),
		flSecurityOpt: opts.NewListOpts(nil),
		flStorageOpt:  opts.NewListOpts(nil),
		flLabelsFile:  opts.NewListOpts(nil),
		flLoggingOpts: opts.NewListOpts(nil),

		flPrivileged:        flags.Bool("privileged", false, "Give extended privileges to this container"),
		flPidMode:           flags.String("pid", "", "PID namespace to use"),
		flUTSMode:           flags.String("uts", "", "UTS namespace to use"),
		flUsernsMode:        flags.String("userns", "", "User namespace to use"),
		flPublishAll:        flags.BoolP("publish-all", "P", false, "Publish all exposed ports to random ports"),
		flStdin:             flags.BoolP("interactive", "i", false, "Keep STDIN open even if not attached"),
		flTty:               flags.BoolP("tty", "t", false, "Allocate a pseudo-TTY"),
		flOomKillDisable:    flags.Bool("oom-kill-disable", false, "Disable OOM Killer"),
		flOomScoreAdj:       flags.Int("oom-score-adj", 0, "Tune host's OOM preferences (-1000 to 1000)"),
		flContainerIDFile:   flags.String("cidfile", "", "Write the container ID to the file"),
		flEntrypoint:        flags.String("entrypoint", "", "Overwrite the default ENTRYPOINT of the image"),
		flHostname:          flags.StringP("hostname", "h", "", "Container host name"),
		flMemoryString:      flags.StringP("memory", "m", "", "Memory limit"),
		flMemoryReservation: flags.String("memory-reservation", "", "Memory soft limit"),
		flMemorySwap:        flags.String("memory-swap", "", "Swap limit equal to memory plus swap: '-1' to enable unlimited swap"),
		flKernelMemory:      flags.String("kernel-memory", "", "Kernel memory limit"),
		flUser:              flags.StringP("user", "u", "", "Username or UID (format: <name|uid>[:<group|gid>])"),
		flWorkingDir:        flags.StringP("workdir", "w", "", "Working directory inside the container"),
		flCPUShares:         flags.Int64P("cpu-shares", "c", 0, "CPU shares (relative weight)"),
		flCPUPercent:        flags.Int64("cpu-percent", 0, "CPU percent (Windows only)"),
		flCPUPeriod:         flags.Int64("cpu-period", 0, "Limit CPU CFS (Completely Fair Scheduler) period"),
		flCPUQuota:          flags.Int64("cpu-quota", 0, "Limit CPU CFS (Completely Fair Scheduler) quota"),
		flCpusetCpus:        flags.String("cpuset-cpus", "", "CPUs in which to allow execution (0-3, 0,1)"),
		flCpusetMems:        flags.String("cpuset-mems", "", "MEMs in which to allow execution (0-3, 0,1)"),
		flBlkioWeight:       flags.Uint16("blkio-weight", 0, "Block IO (relative weight), between 10 and 1000"),
		flIOMaxBandwidth:    flags.String("io-maxbandwidth", "", "Maximum IO bandwidth limit for the system drive (Windows only)"),
		flIOMaxIOps:         flags.Uint64("io-maxiops", 0, "Maximum IOps limit for the system drive (Windows only)"),
		flSwappiness:        flags.Int64("memory-swappiness", -1, "Tune container memory swappiness (0 to 100)"),
		flNetMode:           flags.String("net", "default", "Connect a container to a network"),
		flMacAddress:        flags.String("mac-address", "", "Container MAC address (e.g. 92:d0:c6:0a:29:33)"),
		flIPv4Address:       flags.String("ip", "", "Container IPv4 address (e.g. 172.30.100.104)"),
		flIPv6Address:       flags.String("ip6", "", "Container IPv6 address (e.g. 2001:db8::33)"),
		flIpcMode:           flags.String("ipc", "", "IPC namespace to use"),
		flPidsLimit:         flags.Int64("pids-limit", 0, "Tune container pids limit (set -1 for unlimited)"),
		flRestartPolicy:     flags.String("restart", "no", "Restart policy to apply when a container exits"),
		flReadonlyRootfs:    flags.Bool("read-only", false, "Mount the container's root filesystem as read only"),
		flLoggingDriver:     flags.String("log-driver", "", "Logging driver for container"),
		flCgroupParent:      flags.String("cgroup-parent", "", "Optional parent cgroup for the container"),
		flVolumeDriver:      flags.String("volume-driver", "", "Optional volume driver for the container"),
		flStopSignal:        flags.String("stop-signal", signal.DefaultStopSignal, fmt.Sprintf("Signal to stop a container, %v by default", signal.DefaultStopSignal)),
		flIsolation:         flags.String("isolation", "", "Container isolation technology"),
		flShmSize:           flags.String("shm-size", "", "Size of /dev/shm, default value is 64MB"),
		flNoHealthcheck:     flags.Bool("no-healthcheck", false, "Disable any container-specified HEALTHCHECK"),
		flHealthCmd:         flags.String("health-cmd", "", "Command to run to check health"),
		flHealthInterval:    flags.Duration("health-interval", 0, "Time between running the check"),
		flHealthTimeout:     flags.Duration("health-timeout", 0, "Maximum time to allow one check to run"),
		flHealthRetries:     flags.Int("health-retries", 0, "Consecutive failures needed to report unhealthy"),
	}

	flags.VarP(&copts.flAttach, "attach", "a", "Attach to STDIN, STDOUT or STDERR")
	flags.Var(&copts.flBlkioWeightDevice, "blkio-weight-device", "Block IO weight (relative device weight)")
	flags.Var(&copts.flDeviceReadBps, "device-read-bps", "Limit read rate (bytes per second) from a device")
	flags.Var(&copts.flDeviceWriteBps, "device-write-bps", "Limit write rate (bytes per second) to a device")
	flags.Var(&copts.flDeviceReadIOps, "device-read-iops", "Limit read rate (IO per second) from a device")
	flags.Var(&copts.flDeviceWriteIOps, "device-write-iops", "Limit write rate (IO per second) to a device")
	flags.VarP(&copts.flVolumes, "volume", "v", "Bind mount a volume")
	flags.Var(&copts.flTmpfs, "tmpfs", "Mount a tmpfs directory")
	flags.Var(&copts.flLinks, "link", "Add link to another container")
	flags.Var(&copts.flAliases, "net-alias", "Add network-scoped alias for the container")
	flags.Var(&copts.flDevices, "device", "Add a host device to the container")
	flags.VarP(&copts.flLabels, "label", "l", "Set meta data on a container")
	flags.Var(&copts.flLabelsFile, "label-file", "Read in a line delimited file of labels")
	flags.VarP(&copts.flEnv, "env", "e", "Set environment variables")
	flags.Var(&copts.flEnvFile, "env-file", "Read in a file of environment variables")
	flags.VarP(&copts.flPublish, "publish", "p", "Publish a container's port(s) to the host")
	flags.Var(&copts.flExpose, "expose", "Expose a port or a range of ports")
	flags.Var(&copts.flDNS, "dns", "Set custom DNS servers")
	flags.Var(&copts.flDNSSearch, "dns-search", "Set custom DNS search domains")
	flags.Var(&copts.flDNSOptions, "dns-opt", "Set DNS options")
	flags.Var(&copts.flExtraHosts, "add-host", "Add a custom host-to-IP mapping (host:ip)")
	flags.Var(&copts.flVolumesFrom, "volumes-from", "Mount volumes from the specified container(s)")
	flags.Var(&copts.flCapAdd, "cap-add", "Add Linux capabilities")
	flags.Var(&copts.flCapDrop, "cap-drop", "Drop Linux capabilities")
	flags.Var(&copts.flGroupAdd, "group-add", "Add additional groups to join")
	flags.Var(&copts.flSecurityOpt, "security-opt", "Security Options")
	flags.Var(&copts.flStorageOpt, "storage-opt", "Set storage driver options per container")
	flags.Var(copts.flUlimits, "ulimit", "Ulimit options")
	flags.Var(copts.flSysctls, "sysctl", "Sysctl options")
	flags.Var(&copts.flLoggingOpts, "log-opt", "Log driver options")

	return copts
}

// Parse parses the args for the specified command and generates a Config,
// a HostConfig and returns them with the specified command.
// If the specified args are not valid, it will return an error.
func Parse(flags *pflag.FlagSet, copts *ContainerOptions) (*container.Config, *container.HostConfig, *networktypes.NetworkingConfig, error) {

	var (
		attachStdin  = copts.flAttach.Get("stdin")
		attachStdout = copts.flAttach.Get("stdout")
		attachStderr = copts.flAttach.Get("stderr")
	)

	// Validate the input mac address
	if *copts.flMacAddress != "" {
		if _, err := ValidateMACAddress(*copts.flMacAddress); err != nil {
			return nil, nil, nil, fmt.Errorf("%s is not a valid mac address", *copts.flMacAddress)
		}
	}
	if *copts.flStdin {
		attachStdin = true
	}
	// If -a is not set, attach to stdout and stderr
	if copts.flAttach.Len() == 0 {
		attachStdout = true
		attachStderr = true
	}

	var err error

	var flMemory int64
	if *copts.flMemoryString != "" {
		flMemory, err = units.RAMInBytes(*copts.flMemoryString)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	var MemoryReservation int64
	if *copts.flMemoryReservation != "" {
		MemoryReservation, err = units.RAMInBytes(*copts.flMemoryReservation)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	var memorySwap int64
	if *copts.flMemorySwap != "" {
		if *copts.flMemorySwap == "-1" {
			memorySwap = -1
		} else {
			memorySwap, err = units.RAMInBytes(*copts.flMemorySwap)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}

	var KernelMemory int64
	if *copts.flKernelMemory != "" {
		KernelMemory, err = units.RAMInBytes(*copts.flKernelMemory)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	swappiness := *copts.flSwappiness
	if swappiness != -1 && (swappiness < 0 || swappiness > 100) {
		return nil, nil, nil, fmt.Errorf("invalid value: %d. Valid memory swappiness range is 0-100", swappiness)
	}

	var shmSize int64
	if *copts.flShmSize != "" {
		shmSize, err = units.RAMInBytes(*copts.flShmSize)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	// TODO FIXME units.RAMInBytes should have a uint64 version
	var maxIOBandwidth int64
	if *copts.flIOMaxBandwidth != "" {
		maxIOBandwidth, err = units.RAMInBytes(*copts.flIOMaxBandwidth)
		if err != nil {
			return nil, nil, nil, err
		}
		if maxIOBandwidth < 0 {
			return nil, nil, nil, fmt.Errorf("invalid value: %s. Maximum IO Bandwidth must be positive", *copts.flIOMaxBandwidth)
		}
	}

	var binds []string
	// add any bind targets to the list of container volumes
	for bind := range copts.flVolumes.GetMap() {
		if arr := volumeSplitN(bind, 2); len(arr) > 1 {
			// after creating the bind mount we want to delete it from the copts.flVolumes values because
			// we do not want bind mounts being committed to image configs
			binds = append(binds, bind)
			copts.flVolumes.Delete(bind)
		}
	}

	// Can't evaluate options passed into --tmpfs until we actually mount
	tmpfs := make(map[string]string)
	for _, t := range copts.flTmpfs.GetAll() {
		if arr := strings.SplitN(t, ":", 2); len(arr) > 1 {
			if _, _, err := mount.ParseTmpfsOptions(arr[1]); err != nil {
				return nil, nil, nil, err
			}
			tmpfs[arr[0]] = arr[1]
		} else {
			tmpfs[arr[0]] = ""
		}
	}

	var (
		runCmd     strslice.StrSlice
		entrypoint strslice.StrSlice
	)
	if len(copts.Args) > 0 {
		runCmd = strslice.StrSlice(copts.Args)
	}
	if *copts.flEntrypoint != "" {
		entrypoint = strslice.StrSlice{*copts.flEntrypoint}
	}

	ports, portBindings, err := nat.ParsePortSpecs(copts.flPublish.GetAll())
	if err != nil {
		return nil, nil, nil, err
	}

	// Merge in exposed ports to the map of published ports
	for _, e := range copts.flExpose.GetAll() {
		if strings.Contains(e, ":") {
			return nil, nil, nil, fmt.Errorf("invalid port format for --expose: %s", e)
		}
		//support two formats for expose, original format <portnum>/[<proto>] or <startport-endport>/[<proto>]
		proto, port := nat.SplitProtoPort(e)
		//parse the start and end port and create a sequence of ports to expose
		//if expose a port, the start and end port are the same
		start, end, err := nat.ParsePortRange(port)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("invalid range format for --expose: %s, error: %s", e, err)
		}
		for i := start; i <= end; i++ {
			p, err := nat.NewPort(proto, strconv.FormatUint(i, 10))
			if err != nil {
				return nil, nil, nil, err
			}
			if _, exists := ports[p]; !exists {
				ports[p] = struct{}{}
			}
		}
	}

	// parse device mappings
	deviceMappings := []container.DeviceMapping{}
	for _, device := range copts.flDevices.GetAll() {
		deviceMapping, err := ParseDevice(device)
		if err != nil {
			return nil, nil, nil, err
		}
		deviceMappings = append(deviceMappings, deviceMapping)
	}

	// collect all the environment variables for the container
	envVariables, err := readKVStrings(copts.flEnvFile.GetAll(), copts.flEnv.GetAll())
	if err != nil {
		return nil, nil, nil, err
	}

	// collect all the labels for the container
	labels, err := readKVStrings(copts.flLabelsFile.GetAll(), copts.flLabels.GetAll())
	if err != nil {
		return nil, nil, nil, err
	}

	ipcMode := container.IpcMode(*copts.flIpcMode)
	if !ipcMode.Valid() {
		return nil, nil, nil, fmt.Errorf("--ipc: invalid IPC mode")
	}

	pidMode := container.PidMode(*copts.flPidMode)
	if !pidMode.Valid() {
		return nil, nil, nil, fmt.Errorf("--pid: invalid PID mode")
	}

	utsMode := container.UTSMode(*copts.flUTSMode)
	if !utsMode.Valid() {
		return nil, nil, nil, fmt.Errorf("--uts: invalid UTS mode")
	}

	usernsMode := container.UsernsMode(*copts.flUsernsMode)
	if !usernsMode.Valid() {
		return nil, nil, nil, fmt.Errorf("--userns: invalid USER mode")
	}

	restartPolicy, err := ParseRestartPolicy(*copts.flRestartPolicy)
	if err != nil {
		return nil, nil, nil, err
	}

	loggingOpts, err := parseLoggingOpts(*copts.flLoggingDriver, copts.flLoggingOpts.GetAll())
	if err != nil {
		return nil, nil, nil, err
	}

	securityOpts, err := parseSecurityOpts(copts.flSecurityOpt.GetAll())
	if err != nil {
		return nil, nil, nil, err
	}

	storageOpts, err := parseStorageOpts(copts.flStorageOpt.GetAll())
	if err != nil {
		return nil, nil, nil, err
	}

	// Healthcheck
	var healthConfig *container.HealthConfig
	haveHealthSettings := *copts.flHealthCmd != "" ||
		*copts.flHealthInterval != 0 ||
		*copts.flHealthTimeout != 0 ||
		*copts.flHealthRetries != 0
	if *copts.flNoHealthcheck {
		if haveHealthSettings {
			return nil, nil, nil, fmt.Errorf("--no-healthcheck conflicts with --health-* options")
		}
		test := strslice.StrSlice{"NONE"}
		healthConfig = &container.HealthConfig{Test: test}
	} else if haveHealthSettings {
		var probe strslice.StrSlice
		if *copts.flHealthCmd != "" {
			args := []string{"CMD-SHELL", *copts.flHealthCmd}
			probe = strslice.StrSlice(args)
		}
		if *copts.flHealthInterval < 0 {
			return nil, nil, nil, fmt.Errorf("--health-interval cannot be negative")
		}
		if *copts.flHealthTimeout < 0 {
			return nil, nil, nil, fmt.Errorf("--health-timeout cannot be negative")
		}

		healthConfig = &container.HealthConfig{
			Test:     probe,
			Interval: *copts.flHealthInterval,
			Timeout:  *copts.flHealthTimeout,
			Retries:  *copts.flHealthRetries,
		}
	}

	resources := container.Resources{
		CgroupParent:         *copts.flCgroupParent,
		Memory:               flMemory,
		MemoryReservation:    MemoryReservation,
		MemorySwap:           memorySwap,
		MemorySwappiness:     copts.flSwappiness,
		KernelMemory:         KernelMemory,
		OomKillDisable:       copts.flOomKillDisable,
		CPUPercent:           *copts.flCPUPercent,
		CPUShares:            *copts.flCPUShares,
		CPUPeriod:            *copts.flCPUPeriod,
		CpusetCpus:           *copts.flCpusetCpus,
		CpusetMems:           *copts.flCpusetMems,
		CPUQuota:             *copts.flCPUQuota,
		PidsLimit:            *copts.flPidsLimit,
		BlkioWeight:          *copts.flBlkioWeight,
		BlkioWeightDevice:    copts.flBlkioWeightDevice.GetList(),
		BlkioDeviceReadBps:   copts.flDeviceReadBps.GetList(),
		BlkioDeviceWriteBps:  copts.flDeviceWriteBps.GetList(),
		BlkioDeviceReadIOps:  copts.flDeviceReadIOps.GetList(),
		BlkioDeviceWriteIOps: copts.flDeviceWriteIOps.GetList(),
		IOMaximumIOps:        *copts.flIOMaxIOps,
		IOMaximumBandwidth:   uint64(maxIOBandwidth),
		Ulimits:              copts.flUlimits.GetList(),
		Devices:              deviceMappings,
	}

	config := &container.Config{
		Hostname:     *copts.flHostname,
		ExposedPorts: ports,
		User:         *copts.flUser,
		Tty:          *copts.flTty,
		// TODO: deprecated, it comes from -n, --networking
		// it's still needed internally to set the network to disabled
		// if e.g. bridge is none in daemon opts, and in inspect
		NetworkDisabled: false,
		OpenStdin:       *copts.flStdin,
		AttachStdin:     attachStdin,
		AttachStdout:    attachStdout,
		AttachStderr:    attachStderr,
		Env:             envVariables,
		Cmd:             runCmd,
		Image:           copts.Image,
		Volumes:         copts.flVolumes.GetMap(),
		MacAddress:      *copts.flMacAddress,
		Entrypoint:      entrypoint,
		WorkingDir:      *copts.flWorkingDir,
		Labels:          ConvertKVStringsToMap(labels),
		Healthcheck:     healthConfig,
	}
	if flags.Changed("stop-signal") {
		config.StopSignal = *copts.flStopSignal
	}

	hostConfig := &container.HostConfig{
		Binds:           binds,
		ContainerIDFile: *copts.flContainerIDFile,
		OomScoreAdj:     *copts.flOomScoreAdj,
		Privileged:      *copts.flPrivileged,
		PortBindings:    portBindings,
		Links:           copts.flLinks.GetAll(),
		PublishAllPorts: *copts.flPublishAll,
		// Make sure the dns fields are never nil.
		// New containers don't ever have those fields nil,
		// but pre created containers can still have those nil values.
		// See https://github.com/docker/docker/pull/17779
		// for a more detailed explanation on why we don't want that.
		DNS:            copts.flDNS.GetAllOrEmpty(),
		DNSSearch:      copts.flDNSSearch.GetAllOrEmpty(),
		DNSOptions:     copts.flDNSOptions.GetAllOrEmpty(),
		ExtraHosts:     copts.flExtraHosts.GetAll(),
		VolumesFrom:    copts.flVolumesFrom.GetAll(),
		NetworkMode:    container.NetworkMode(*copts.flNetMode),
		IpcMode:        ipcMode,
		PidMode:        pidMode,
		UTSMode:        utsMode,
		UsernsMode:     usernsMode,
		CapAdd:         strslice.StrSlice(copts.flCapAdd.GetAll()),
		CapDrop:        strslice.StrSlice(copts.flCapDrop.GetAll()),
		GroupAdd:       copts.flGroupAdd.GetAll(),
		RestartPolicy:  restartPolicy,
		SecurityOpt:    securityOpts,
		StorageOpt:     storageOpts,
		ReadonlyRootfs: *copts.flReadonlyRootfs,
		LogConfig:      container.LogConfig{Type: *copts.flLoggingDriver, Config: loggingOpts},
		VolumeDriver:   *copts.flVolumeDriver,
		Isolation:      container.Isolation(*copts.flIsolation),
		ShmSize:        shmSize,
		Resources:      resources,
		Tmpfs:          tmpfs,
		Sysctls:        copts.flSysctls.GetAll(),
	}

	// When allocating stdin in attached mode, close stdin at client disconnect
	if config.OpenStdin && config.AttachStdin {
		config.StdinOnce = true
	}

	networkingConfig := &networktypes.NetworkingConfig{
		EndpointsConfig: make(map[string]*networktypes.EndpointSettings),
	}

	if *copts.flIPv4Address != "" || *copts.flIPv6Address != "" {
		networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)] = &networktypes.EndpointSettings{
			IPAMConfig: &networktypes.EndpointIPAMConfig{
				IPv4Address: *copts.flIPv4Address,
				IPv6Address: *copts.flIPv6Address,
			},
		}
	}

	if hostConfig.NetworkMode.IsUserDefined() && len(hostConfig.Links) > 0 {
		epConfig := networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)]
		if epConfig == nil {
			epConfig = &networktypes.EndpointSettings{}
		}
		epConfig.Links = make([]string, len(hostConfig.Links))
		copy(epConfig.Links, hostConfig.Links)
		networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)] = epConfig
	}

	if copts.flAliases.Len() > 0 {
		epConfig := networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)]
		if epConfig == nil {
			epConfig = &networktypes.EndpointSettings{}
		}
		epConfig.Aliases = make([]string, copts.flAliases.Len())
		copy(epConfig.Aliases, copts.flAliases.GetAll())
		networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)] = epConfig
	}

	return config, hostConfig, networkingConfig, nil
}

// reads a file of line terminated key=value pairs, and overrides any keys
// present in the file with additional pairs specified in the override parameter
func readKVStrings(files []string, override []string) ([]string, error) {
	envVariables := []string{}
	for _, ef := range files {
		parsedVars, err := ParseEnvFile(ef)
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
		return map[string]string{}, fmt.Errorf("invalid logging opts for driver %s", loggingDriver)
	}
	return loggingOptsMap, nil
}

// takes a local seccomp daemon, reads the file contents for sending to the daemon
func parseSecurityOpts(securityOpts []string) ([]string, error) {
	for key, opt := range securityOpts {
		con := strings.SplitN(opt, "=", 2)
		if len(con) == 1 && con[0] != "no-new-privileges" {
			if strings.Index(opt, ":") != -1 {
				con = strings.SplitN(opt, ":", 2)
			} else {
				return securityOpts, fmt.Errorf("Invalid --security-opt: %q", opt)
			}
		}
		if con[0] == "seccomp" && con[1] != "unconfined" {
			f, err := ioutil.ReadFile(con[1])
			if err != nil {
				return securityOpts, fmt.Errorf("opening seccomp profile (%s) failed: %v", con[1], err)
			}
			b := bytes.NewBuffer(nil)
			if err := json.Compact(b, f); err != nil {
				return securityOpts, fmt.Errorf("compacting json for seccomp profile (%s) failed: %v", con[1], err)
			}
			securityOpts[key] = fmt.Sprintf("seccomp=%s", b.Bytes())
		}
	}

	return securityOpts, nil
}

// parses storage options per container into a map
func parseStorageOpts(storageOpts []string) (map[string]string, error) {
	m := make(map[string]string)
	for _, option := range storageOpts {
		if strings.Contains(option, "=") {
			opt := strings.SplitN(option, "=", 2)
			m[opt[0]] = opt[1]
		} else {
			return nil, fmt.Errorf("Invalid storage option.")
		}
	}
	return m, nil
}

// ParseRestartPolicy returns the parsed policy or an error indicating what is incorrect
func ParseRestartPolicy(policy string) (container.RestartPolicy, error) {
	p := container.RestartPolicy{}

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

// ParseDevice parses a device mapping string to a container.DeviceMapping struct
func ParseDevice(device string) (container.DeviceMapping, error) {
	src := ""
	dst := ""
	permissions := "rwm"
	arr := strings.Split(device, ":")
	switch len(arr) {
	case 3:
		permissions = arr[2]
		fallthrough
	case 2:
		if ValidDeviceMode(arr[1]) {
			permissions = arr[1]
		} else {
			dst = arr[1]
		}
		fallthrough
	case 1:
		src = arr[0]
	default:
		return container.DeviceMapping{}, fmt.Errorf("invalid device specification: %s", device)
	}

	if dst == "" {
		dst = src
	}

	deviceMapping := container.DeviceMapping{
		PathOnHost:        src,
		PathInContainer:   dst,
		CgroupPermissions: permissions,
	}
	return deviceMapping, nil
}

// ParseLink parses and validates the specified string as a link format (name:alias)
func ParseLink(val string) (string, string, error) {
	if val == "" {
		return "", "", fmt.Errorf("empty string specified for links")
	}
	arr := strings.Split(val, ":")
	if len(arr) > 2 {
		return "", "", fmt.Errorf("bad format for links: %s", val)
	}
	if len(arr) == 1 {
		return val, val, nil
	}
	// This is kept because we can actually get a HostConfig with links
	// from an already created container and the format is not `foo:bar`
	// but `/foo:/c1/bar`
	if strings.HasPrefix(arr[0], "/") {
		_, alias := path.Split(arr[1])
		return arr[0][1:], alias, nil
	}
	return arr[0], arr[1], nil
}

// ValidateLink validates that the specified string has a valid link format (containerName:alias).
func ValidateLink(val string) (string, error) {
	if _, _, err := ParseLink(val); err != nil {
		return val, err
	}
	return val, nil
}

// ValidDeviceMode checks if the mode for device is valid or not.
// Valid mode is a composition of r (read), w (write), and m (mknod).
func ValidDeviceMode(mode string) bool {
	var legalDeviceMode = map[rune]bool{
		'r': true,
		'w': true,
		'm': true,
	}
	if mode == "" {
		return false
	}
	for _, c := range mode {
		if !legalDeviceMode[c] {
			return false
		}
		legalDeviceMode[c] = false
	}
	return true
}

// ValidateDevice validates a path for devices
// It will make sure 'val' is in the form:
//    [host-dir:]container-path[:mode]
// It also validates the device mode.
func ValidateDevice(val string) (string, error) {
	return validatePath(val, ValidDeviceMode)
}

func validatePath(val string, validator func(string) bool) (string, error) {
	var containerPath string
	var mode string

	if strings.Count(val, ":") > 2 {
		return val, fmt.Errorf("bad format for path: %s", val)
	}

	split := strings.SplitN(val, ":", 3)
	if split[0] == "" {
		return val, fmt.Errorf("bad format for path: %s", val)
	}
	switch len(split) {
	case 1:
		containerPath = split[0]
		val = path.Clean(containerPath)
	case 2:
		if isValid := validator(split[1]); isValid {
			containerPath = split[0]
			mode = split[1]
			val = fmt.Sprintf("%s:%s", path.Clean(containerPath), mode)
		} else {
			containerPath = split[1]
			val = fmt.Sprintf("%s:%s", split[0], path.Clean(containerPath))
		}
	case 3:
		containerPath = split[1]
		mode = split[2]
		if isValid := validator(split[2]); !isValid {
			return val, fmt.Errorf("bad mode specified: %s", mode)
		}
		val = fmt.Sprintf("%s:%s:%s", split[0], containerPath, mode)
	}

	if !path.IsAbs(containerPath) {
		return val, fmt.Errorf("%s is not an absolute path", containerPath)
	}
	return val, nil
}

// volumeSplitN splits raw into a maximum of n parts, separated by a separator colon.
// A separator colon is the last `:` character in the regex `[:\\]?[a-zA-Z]:` (note `\\` is `\` escaped).
// In Windows driver letter appears in two situations:
// a. `^[a-zA-Z]:` (A colon followed  by `^[a-zA-Z]:` is OK as colon is the separator in volume option)
// b. A string in the format like `\\?\C:\Windows\...` (UNC).
// Therefore, a driver letter can only follow either a `:` or `\\`
// This allows to correctly split strings such as `C:\foo:D:\:rw` or `/tmp/q:/foo`.
func volumeSplitN(raw string, n int) []string {
	var array []string
	if len(raw) == 0 || raw[0] == ':' {
		// invalid
		return nil
	}
	// numberOfParts counts the number of parts separated by a separator colon
	numberOfParts := 0
	// left represents the left-most cursor in raw, updated at every `:` character considered as a separator.
	left := 0
	// right represents the right-most cursor in raw incremented with the loop. Note this
	// starts at index 1 as index 0 is already handle above as a special case.
	for right := 1; right < len(raw); right++ {
		// stop parsing if reached maximum number of parts
		if n >= 0 && numberOfParts >= n {
			break
		}
		if raw[right] != ':' {
			continue
		}
		potentialDriveLetter := raw[right-1]
		if (potentialDriveLetter >= 'A' && potentialDriveLetter <= 'Z') || (potentialDriveLetter >= 'a' && potentialDriveLetter <= 'z') {
			if right > 1 {
				beforePotentialDriveLetter := raw[right-2]
				// Only `:` or `\\` are checked (`/` could fall into the case of `/tmp/q:/foo`)
				if beforePotentialDriveLetter != ':' && beforePotentialDriveLetter != '\\' {
					// e.g. `C:` is not preceded by any delimiter, therefore it was not a drive letter but a path ending with `C:`.
					array = append(array, raw[left:right])
					left = right + 1
					numberOfParts++
				}
				// else, `C:` is considered as a drive letter and not as a delimiter, so we continue parsing.
			}
			// if right == 1, then `C:` is the beginning of the raw string, therefore `:` is again not considered a delimiter and we continue parsing.
		} else {
			// if `:` is not preceded by a potential drive letter, then consider it as a delimiter.
			array = append(array, raw[left:right])
			left = right + 1
			numberOfParts++
		}
	}
	// need to take care of the last part
	if left < len(raw) {
		if n >= 0 && numberOfParts >= n {
			// if the maximum number of parts is reached, just append the rest to the last part
			// left-1 is at the last `:` that needs to be included since not considered a separator.
			array[n-1] += raw[left-1:]
		} else {
			array = append(array, raw[left:])
		}
	}
	return array
}
