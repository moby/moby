package container

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/signal"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/go-connections/nat"
	units "github.com/docker/go-units"
	"github.com/spf13/pflag"
)

// containerOptions is a data object with all the options for creating a container
type containerOptions struct {
	attach             opts.ListOpts
	volumes            opts.ListOpts
	tmpfs              opts.ListOpts
	blkioWeightDevice  opts.WeightdeviceOpt
	deviceReadBps      opts.ThrottledeviceOpt
	deviceWriteBps     opts.ThrottledeviceOpt
	links              opts.ListOpts
	aliases            opts.ListOpts
	linkLocalIPs       opts.ListOpts
	deviceReadIOps     opts.ThrottledeviceOpt
	deviceWriteIOps    opts.ThrottledeviceOpt
	env                opts.ListOpts
	labels             opts.ListOpts
	devices            opts.ListOpts
	ulimits            *opts.UlimitOpt
	sysctls            *opts.MapOpts
	publish            opts.ListOpts
	expose             opts.ListOpts
	dns                opts.ListOpts
	dnsSearch          opts.ListOpts
	dnsOptions         opts.ListOpts
	extraHosts         opts.ListOpts
	volumesFrom        opts.ListOpts
	envFile            opts.ListOpts
	capAdd             opts.ListOpts
	capDrop            opts.ListOpts
	groupAdd           opts.ListOpts
	securityOpt        opts.ListOpts
	storageOpt         opts.ListOpts
	labelsFile         opts.ListOpts
	loggingOpts        opts.ListOpts
	privileged         bool
	pidMode            string
	utsMode            string
	usernsMode         string
	publishAll         bool
	stdin              bool
	tty                bool
	oomKillDisable     bool
	oomScoreAdj        int
	containerIDFile    string
	entrypoint         string
	hostname           string
	memoryString       string
	memoryReservation  string
	memorySwap         string
	kernelMemory       string
	user               string
	workingDir         string
	cpuCount           int64
	cpuShares          int64
	cpuPercent         int64
	cpuPeriod          int64
	cpuRealtimePeriod  int64
	cpuRealtimeRuntime int64
	cpuQuota           int64
	cpus               opts.NanoCPUs
	cpusetCpus         string
	cpusetMems         string
	blkioWeight        uint16
	ioMaxBandwidth     string
	ioMaxIOps          uint64
	swappiness         int64
	netMode            string
	macAddress         string
	ipv4Address        string
	ipv6Address        string
	ipcMode            string
	pidsLimit          int64
	restartPolicy      string
	readonlyRootfs     bool
	loggingDriver      string
	cgroupParent       string
	volumeDriver       string
	stopSignal         string
	stopTimeout        int
	isolation          string
	shmSize            string
	noHealthcheck      bool
	healthCmd          string
	healthInterval     time.Duration
	healthTimeout      time.Duration
	healthRetries      int
	runtime            string
	autoRemove         bool
	init               bool
	initPath           string
	credentialSpec     string

	Image string
	Args  []string
}

// addFlags adds all command line flags that will be used by parse to the FlagSet
func addFlags(flags *pflag.FlagSet) *containerOptions {
	copts := &containerOptions{
		aliases:           opts.NewListOpts(nil),
		attach:            opts.NewListOpts(validateAttach),
		blkioWeightDevice: opts.NewWeightdeviceOpt(opts.ValidateWeightDevice),
		capAdd:            opts.NewListOpts(nil),
		capDrop:           opts.NewListOpts(nil),
		dns:               opts.NewListOpts(opts.ValidateIPAddress),
		dnsOptions:        opts.NewListOpts(nil),
		dnsSearch:         opts.NewListOpts(opts.ValidateDNSSearch),
		deviceReadBps:     opts.NewThrottledeviceOpt(opts.ValidateThrottleBpsDevice),
		deviceReadIOps:    opts.NewThrottledeviceOpt(opts.ValidateThrottleIOpsDevice),
		deviceWriteBps:    opts.NewThrottledeviceOpt(opts.ValidateThrottleBpsDevice),
		deviceWriteIOps:   opts.NewThrottledeviceOpt(opts.ValidateThrottleIOpsDevice),
		devices:           opts.NewListOpts(validateDevice),
		env:               opts.NewListOpts(opts.ValidateEnv),
		envFile:           opts.NewListOpts(nil),
		expose:            opts.NewListOpts(nil),
		extraHosts:        opts.NewListOpts(opts.ValidateExtraHost),
		groupAdd:          opts.NewListOpts(nil),
		labels:            opts.NewListOpts(opts.ValidateEnv),
		labelsFile:        opts.NewListOpts(nil),
		linkLocalIPs:      opts.NewListOpts(nil),
		links:             opts.NewListOpts(opts.ValidateLink),
		loggingOpts:       opts.NewListOpts(nil),
		publish:           opts.NewListOpts(nil),
		securityOpt:       opts.NewListOpts(nil),
		storageOpt:        opts.NewListOpts(nil),
		sysctls:           opts.NewMapOpts(nil, opts.ValidateSysctl),
		tmpfs:             opts.NewListOpts(nil),
		ulimits:           opts.NewUlimitOpt(nil),
		volumes:           opts.NewListOpts(nil),
		volumesFrom:       opts.NewListOpts(nil),
	}

	// General purpose flags
	flags.VarP(&copts.attach, "attach", "a", "Attach to STDIN, STDOUT or STDERR")
	flags.Var(&copts.devices, "device", "Add a host device to the container")
	flags.VarP(&copts.env, "env", "e", "Set environment variables")
	flags.Var(&copts.envFile, "env-file", "Read in a file of environment variables")
	flags.StringVar(&copts.entrypoint, "entrypoint", "", "Overwrite the default ENTRYPOINT of the image")
	flags.Var(&copts.groupAdd, "group-add", "Add additional groups to join")
	flags.StringVarP(&copts.hostname, "hostname", "h", "", "Container host name")
	flags.BoolVarP(&copts.stdin, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.VarP(&copts.labels, "label", "l", "Set meta data on a container")
	flags.Var(&copts.labelsFile, "label-file", "Read in a line delimited file of labels")
	flags.BoolVar(&copts.readonlyRootfs, "read-only", false, "Mount the container's root filesystem as read only")
	flags.StringVar(&copts.restartPolicy, "restart", "no", "Restart policy to apply when a container exits")
	flags.StringVar(&copts.stopSignal, "stop-signal", signal.DefaultStopSignal, fmt.Sprintf("Signal to stop a container, %v by default", signal.DefaultStopSignal))
	flags.IntVar(&copts.stopTimeout, "stop-timeout", 0, "Timeout (in seconds) to stop a container")
	flags.SetAnnotation("stop-timeout", "version", []string{"1.25"})
	flags.Var(copts.sysctls, "sysctl", "Sysctl options")
	flags.BoolVarP(&copts.tty, "tty", "t", false, "Allocate a pseudo-TTY")
	flags.Var(copts.ulimits, "ulimit", "Ulimit options")
	flags.StringVarP(&copts.user, "user", "u", "", "Username or UID (format: <name|uid>[:<group|gid>])")
	flags.StringVarP(&copts.workingDir, "workdir", "w", "", "Working directory inside the container")
	flags.BoolVar(&copts.autoRemove, "rm", false, "Automatically remove the container when it exits")

	// Security
	flags.Var(&copts.capAdd, "cap-add", "Add Linux capabilities")
	flags.Var(&copts.capDrop, "cap-drop", "Drop Linux capabilities")
	flags.BoolVar(&copts.privileged, "privileged", false, "Give extended privileges to this container")
	flags.Var(&copts.securityOpt, "security-opt", "Security Options")
	flags.StringVar(&copts.usernsMode, "userns", "", "User namespace to use")
	flags.StringVar(&copts.credentialSpec, "credentialspec", "", "Credential spec for managed service account (Windows only)")

	// Network and port publishing flag
	flags.Var(&copts.extraHosts, "add-host", "Add a custom host-to-IP mapping (host:ip)")
	flags.Var(&copts.dns, "dns", "Set custom DNS servers")
	// We allow for both "--dns-opt" and "--dns-option", although the latter is the recommended way.
	// This is to be consistent with service create/update
	flags.Var(&copts.dnsOptions, "dns-opt", "Set DNS options")
	flags.Var(&copts.dnsOptions, "dns-option", "Set DNS options")
	flags.MarkHidden("dns-opt")
	flags.Var(&copts.dnsSearch, "dns-search", "Set custom DNS search domains")
	flags.Var(&copts.expose, "expose", "Expose a port or a range of ports")
	flags.StringVar(&copts.ipv4Address, "ip", "", "IPv4 address (e.g., 172.30.100.104)")
	flags.StringVar(&copts.ipv6Address, "ip6", "", "IPv6 address (e.g., 2001:db8::33)")
	flags.Var(&copts.links, "link", "Add link to another container")
	flags.Var(&copts.linkLocalIPs, "link-local-ip", "Container IPv4/IPv6 link-local addresses")
	flags.StringVar(&copts.macAddress, "mac-address", "", "Container MAC address (e.g., 92:d0:c6:0a:29:33)")
	flags.VarP(&copts.publish, "publish", "p", "Publish a container's port(s) to the host")
	flags.BoolVarP(&copts.publishAll, "publish-all", "P", false, "Publish all exposed ports to random ports")
	// We allow for both "--net" and "--network", although the latter is the recommended way.
	flags.StringVar(&copts.netMode, "net", "default", "Connect a container to a network")
	flags.StringVar(&copts.netMode, "network", "default", "Connect a container to a network")
	flags.MarkHidden("net")
	// We allow for both "--net-alias" and "--network-alias", although the latter is the recommended way.
	flags.Var(&copts.aliases, "net-alias", "Add network-scoped alias for the container")
	flags.Var(&copts.aliases, "network-alias", "Add network-scoped alias for the container")
	flags.MarkHidden("net-alias")

	// Logging and storage
	flags.StringVar(&copts.loggingDriver, "log-driver", "", "Logging driver for the container")
	flags.StringVar(&copts.volumeDriver, "volume-driver", "", "Optional volume driver for the container")
	flags.Var(&copts.loggingOpts, "log-opt", "Log driver options")
	flags.Var(&copts.storageOpt, "storage-opt", "Storage driver options for the container")
	flags.Var(&copts.tmpfs, "tmpfs", "Mount a tmpfs directory")
	flags.Var(&copts.volumesFrom, "volumes-from", "Mount volumes from the specified container(s)")
	flags.VarP(&copts.volumes, "volume", "v", "Bind mount a volume")

	// Health-checking
	flags.StringVar(&copts.healthCmd, "health-cmd", "", "Command to run to check health")
	flags.DurationVar(&copts.healthInterval, "health-interval", 0, "Time between running the check (ns|us|ms|s|m|h) (default 0s)")
	flags.IntVar(&copts.healthRetries, "health-retries", 0, "Consecutive failures needed to report unhealthy")
	flags.DurationVar(&copts.healthTimeout, "health-timeout", 0, "Maximum time to allow one check to run (ns|us|ms|s|m|h) (default 0s)")
	flags.BoolVar(&copts.noHealthcheck, "no-healthcheck", false, "Disable any container-specified HEALTHCHECK")

	// Resource management
	flags.Uint16Var(&copts.blkioWeight, "blkio-weight", 0, "Block IO (relative weight), between 10 and 1000, or 0 to disable (default 0)")
	flags.Var(&copts.blkioWeightDevice, "blkio-weight-device", "Block IO weight (relative device weight)")
	flags.StringVar(&copts.containerIDFile, "cidfile", "", "Write the container ID to the file")
	flags.StringVar(&copts.cpusetCpus, "cpuset-cpus", "", "CPUs in which to allow execution (0-3, 0,1)")
	flags.StringVar(&copts.cpusetMems, "cpuset-mems", "", "MEMs in which to allow execution (0-3, 0,1)")
	flags.Int64Var(&copts.cpuCount, "cpu-count", 0, "CPU count (Windows only)")
	flags.Int64Var(&copts.cpuPercent, "cpu-percent", 0, "CPU percent (Windows only)")
	flags.Int64Var(&copts.cpuPeriod, "cpu-period", 0, "Limit CPU CFS (Completely Fair Scheduler) period")
	flags.Int64Var(&copts.cpuQuota, "cpu-quota", 0, "Limit CPU CFS (Completely Fair Scheduler) quota")
	flags.Int64Var(&copts.cpuRealtimePeriod, "cpu-rt-period", 0, "Limit CPU real-time period in microseconds")
	flags.Int64Var(&copts.cpuRealtimeRuntime, "cpu-rt-runtime", 0, "Limit CPU real-time runtime in microseconds")
	flags.Int64VarP(&copts.cpuShares, "cpu-shares", "c", 0, "CPU shares (relative weight)")
	flags.Var(&copts.cpus, "cpus", "Number of CPUs")
	flags.Var(&copts.deviceReadBps, "device-read-bps", "Limit read rate (bytes per second) from a device")
	flags.Var(&copts.deviceReadIOps, "device-read-iops", "Limit read rate (IO per second) from a device")
	flags.Var(&copts.deviceWriteBps, "device-write-bps", "Limit write rate (bytes per second) to a device")
	flags.Var(&copts.deviceWriteIOps, "device-write-iops", "Limit write rate (IO per second) to a device")
	flags.StringVar(&copts.ioMaxBandwidth, "io-maxbandwidth", "", "Maximum IO bandwidth limit for the system drive (Windows only)")
	flags.Uint64Var(&copts.ioMaxIOps, "io-maxiops", 0, "Maximum IOps limit for the system drive (Windows only)")
	flags.StringVar(&copts.kernelMemory, "kernel-memory", "", "Kernel memory limit")
	flags.StringVarP(&copts.memoryString, "memory", "m", "", "Memory limit")
	flags.StringVar(&copts.memoryReservation, "memory-reservation", "", "Memory soft limit")
	flags.StringVar(&copts.memorySwap, "memory-swap", "", "Swap limit equal to memory plus swap: '-1' to enable unlimited swap")
	flags.Int64Var(&copts.swappiness, "memory-swappiness", -1, "Tune container memory swappiness (0 to 100)")
	flags.BoolVar(&copts.oomKillDisable, "oom-kill-disable", false, "Disable OOM Killer")
	flags.IntVar(&copts.oomScoreAdj, "oom-score-adj", 0, "Tune host's OOM preferences (-1000 to 1000)")
	flags.Int64Var(&copts.pidsLimit, "pids-limit", 0, "Tune container pids limit (set -1 for unlimited)")

	// Low-level execution (cgroups, namespaces, ...)
	flags.StringVar(&copts.cgroupParent, "cgroup-parent", "", "Optional parent cgroup for the container")
	flags.StringVar(&copts.ipcMode, "ipc", "", "IPC namespace to use")
	flags.StringVar(&copts.isolation, "isolation", "", "Container isolation technology")
	flags.StringVar(&copts.pidMode, "pid", "", "PID namespace to use")
	flags.StringVar(&copts.shmSize, "shm-size", "", "Size of /dev/shm, default value is 64MB")
	flags.StringVar(&copts.utsMode, "uts", "", "UTS namespace to use")
	flags.StringVar(&copts.runtime, "runtime", "", "Runtime to use for this container")

	flags.BoolVar(&copts.init, "init", false, "Run an init inside the container that forwards signals and reaps processes")
	flags.StringVar(&copts.initPath, "init-path", "", "Path to the docker-init binary")
	return copts
}

// parse parses the args for the specified command and generates a Config,
// a HostConfig and returns them with the specified command.
// If the specified args are not valid, it will return an error.
func parse(flags *pflag.FlagSet, copts *containerOptions) (*container.Config, *container.HostConfig, *networktypes.NetworkingConfig, error) {
	var (
		attachStdin  = copts.attach.Get("stdin")
		attachStdout = copts.attach.Get("stdout")
		attachStderr = copts.attach.Get("stderr")
	)

	// Validate the input mac address
	if copts.macAddress != "" {
		if _, err := opts.ValidateMACAddress(copts.macAddress); err != nil {
			return nil, nil, nil, fmt.Errorf("%s is not a valid mac address", copts.macAddress)
		}
	}
	if copts.stdin {
		attachStdin = true
	}
	// If -a is not set, attach to stdout and stderr
	if copts.attach.Len() == 0 {
		attachStdout = true
		attachStderr = true
	}

	var err error

	var memory int64
	if copts.memoryString != "" {
		memory, err = units.RAMInBytes(copts.memoryString)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	var memoryReservation int64
	if copts.memoryReservation != "" {
		memoryReservation, err = units.RAMInBytes(copts.memoryReservation)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	var memorySwap int64
	if copts.memorySwap != "" {
		if copts.memorySwap == "-1" {
			memorySwap = -1
		} else {
			memorySwap, err = units.RAMInBytes(copts.memorySwap)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}

	var kernelMemory int64
	if copts.kernelMemory != "" {
		kernelMemory, err = units.RAMInBytes(copts.kernelMemory)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	swappiness := copts.swappiness
	if swappiness != -1 && (swappiness < 0 || swappiness > 100) {
		return nil, nil, nil, fmt.Errorf("invalid value: %d. Valid memory swappiness range is 0-100", swappiness)
	}

	var shmSize int64
	if copts.shmSize != "" {
		shmSize, err = units.RAMInBytes(copts.shmSize)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	// TODO FIXME units.RAMInBytes should have a uint64 version
	var maxIOBandwidth int64
	if copts.ioMaxBandwidth != "" {
		maxIOBandwidth, err = units.RAMInBytes(copts.ioMaxBandwidth)
		if err != nil {
			return nil, nil, nil, err
		}
		if maxIOBandwidth < 0 {
			return nil, nil, nil, fmt.Errorf("invalid value: %s. Maximum IO Bandwidth must be positive", copts.ioMaxBandwidth)
		}
	}

	var binds []string
	volumes := copts.volumes.GetMap()
	// add any bind targets to the list of container volumes
	for bind := range copts.volumes.GetMap() {
		if arr := volumeSplitN(bind, 2); len(arr) > 1 {
			// after creating the bind mount we want to delete it from the copts.volumes values because
			// we do not want bind mounts being committed to image configs
			binds = append(binds, bind)
			// We should delete from the map (`volumes`) here, as deleting from copts.volumes will not work if
			// there are duplicates entries.
			delete(volumes, bind)
		}
	}

	// Can't evaluate options passed into --tmpfs until we actually mount
	tmpfs := make(map[string]string)
	for _, t := range copts.tmpfs.GetAll() {
		if arr := strings.SplitN(t, ":", 2); len(arr) > 1 {
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

	if copts.entrypoint != "" {
		entrypoint = strslice.StrSlice{copts.entrypoint}
	} else if flags.Changed("entrypoint") {
		// if `--entrypoint=` is parsed then Entrypoint is reset
		entrypoint = []string{""}
	}

	ports, portBindings, err := nat.ParsePortSpecs(copts.publish.GetAll())
	if err != nil {
		return nil, nil, nil, err
	}

	// Merge in exposed ports to the map of published ports
	for _, e := range copts.expose.GetAll() {
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
	for _, device := range copts.devices.GetAll() {
		deviceMapping, err := parseDevice(device)
		if err != nil {
			return nil, nil, nil, err
		}
		deviceMappings = append(deviceMappings, deviceMapping)
	}

	// collect all the environment variables for the container
	envVariables, err := runconfigopts.ReadKVStrings(copts.envFile.GetAll(), copts.env.GetAll())
	if err != nil {
		return nil, nil, nil, err
	}

	// collect all the labels for the container
	labels, err := runconfigopts.ReadKVStrings(copts.labelsFile.GetAll(), copts.labels.GetAll())
	if err != nil {
		return nil, nil, nil, err
	}

	ipcMode := container.IpcMode(copts.ipcMode)
	if !ipcMode.Valid() {
		return nil, nil, nil, fmt.Errorf("--ipc: invalid IPC mode")
	}

	pidMode := container.PidMode(copts.pidMode)
	if !pidMode.Valid() {
		return nil, nil, nil, fmt.Errorf("--pid: invalid PID mode")
	}

	utsMode := container.UTSMode(copts.utsMode)
	if !utsMode.Valid() {
		return nil, nil, nil, fmt.Errorf("--uts: invalid UTS mode")
	}

	usernsMode := container.UsernsMode(copts.usernsMode)
	if !usernsMode.Valid() {
		return nil, nil, nil, fmt.Errorf("--userns: invalid USER mode")
	}

	restartPolicy, err := runconfigopts.ParseRestartPolicy(copts.restartPolicy)
	if err != nil {
		return nil, nil, nil, err
	}

	loggingOpts, err := parseLoggingOpts(copts.loggingDriver, copts.loggingOpts.GetAll())
	if err != nil {
		return nil, nil, nil, err
	}

	securityOpts, err := parseSecurityOpts(copts.securityOpt.GetAll())
	if err != nil {
		return nil, nil, nil, err
	}

	storageOpts, err := parseStorageOpts(copts.storageOpt.GetAll())
	if err != nil {
		return nil, nil, nil, err
	}

	// Healthcheck
	var healthConfig *container.HealthConfig
	haveHealthSettings := copts.healthCmd != "" ||
		copts.healthInterval != 0 ||
		copts.healthTimeout != 0 ||
		copts.healthRetries != 0
	if copts.noHealthcheck {
		if haveHealthSettings {
			return nil, nil, nil, fmt.Errorf("--no-healthcheck conflicts with --health-* options")
		}
		test := strslice.StrSlice{"NONE"}
		healthConfig = &container.HealthConfig{Test: test}
	} else if haveHealthSettings {
		var probe strslice.StrSlice
		if copts.healthCmd != "" {
			args := []string{"CMD-SHELL", copts.healthCmd}
			probe = strslice.StrSlice(args)
		}
		if copts.healthInterval < 0 {
			return nil, nil, nil, fmt.Errorf("--health-interval cannot be negative")
		}
		if copts.healthTimeout < 0 {
			return nil, nil, nil, fmt.Errorf("--health-timeout cannot be negative")
		}

		healthConfig = &container.HealthConfig{
			Test:     probe,
			Interval: copts.healthInterval,
			Timeout:  copts.healthTimeout,
			Retries:  copts.healthRetries,
		}
	}

	resources := container.Resources{
		CgroupParent:         copts.cgroupParent,
		Memory:               memory,
		MemoryReservation:    memoryReservation,
		MemorySwap:           memorySwap,
		MemorySwappiness:     &copts.swappiness,
		KernelMemory:         kernelMemory,
		OomKillDisable:       &copts.oomKillDisable,
		NanoCPUs:             copts.cpus.Value(),
		CPUCount:             copts.cpuCount,
		CPUPercent:           copts.cpuPercent,
		CPUShares:            copts.cpuShares,
		CPUPeriod:            copts.cpuPeriod,
		CpusetCpus:           copts.cpusetCpus,
		CpusetMems:           copts.cpusetMems,
		CPUQuota:             copts.cpuQuota,
		CPURealtimePeriod:    copts.cpuRealtimePeriod,
		CPURealtimeRuntime:   copts.cpuRealtimeRuntime,
		PidsLimit:            copts.pidsLimit,
		BlkioWeight:          copts.blkioWeight,
		BlkioWeightDevice:    copts.blkioWeightDevice.GetList(),
		BlkioDeviceReadBps:   copts.deviceReadBps.GetList(),
		BlkioDeviceWriteBps:  copts.deviceWriteBps.GetList(),
		BlkioDeviceReadIOps:  copts.deviceReadIOps.GetList(),
		BlkioDeviceWriteIOps: copts.deviceWriteIOps.GetList(),
		IOMaximumIOps:        copts.ioMaxIOps,
		IOMaximumBandwidth:   uint64(maxIOBandwidth),
		Ulimits:              copts.ulimits.GetList(),
		Devices:              deviceMappings,
	}

	config := &container.Config{
		Hostname:     copts.hostname,
		ExposedPorts: ports,
		User:         copts.user,
		Tty:          copts.tty,
		// TODO: deprecated, it comes from -n, --networking
		// it's still needed internally to set the network to disabled
		// if e.g. bridge is none in daemon opts, and in inspect
		NetworkDisabled: false,
		OpenStdin:       copts.stdin,
		AttachStdin:     attachStdin,
		AttachStdout:    attachStdout,
		AttachStderr:    attachStderr,
		Env:             envVariables,
		Cmd:             runCmd,
		Image:           copts.Image,
		Volumes:         volumes,
		MacAddress:      copts.macAddress,
		Entrypoint:      entrypoint,
		WorkingDir:      copts.workingDir,
		Labels:          runconfigopts.ConvertKVStringsToMap(labels),
		Healthcheck:     healthConfig,
	}
	if flags.Changed("stop-signal") {
		config.StopSignal = copts.stopSignal
	}
	if flags.Changed("stop-timeout") {
		config.StopTimeout = &copts.stopTimeout
	}

	hostConfig := &container.HostConfig{
		Binds:           binds,
		ContainerIDFile: copts.containerIDFile,
		OomScoreAdj:     copts.oomScoreAdj,
		AutoRemove:      copts.autoRemove,
		Privileged:      copts.privileged,
		PortBindings:    portBindings,
		Links:           copts.links.GetAll(),
		PublishAllPorts: copts.publishAll,
		// Make sure the dns fields are never nil.
		// New containers don't ever have those fields nil,
		// but pre created containers can still have those nil values.
		// See https://github.com/docker/docker/pull/17779
		// for a more detailed explanation on why we don't want that.
		DNS:            copts.dns.GetAllOrEmpty(),
		DNSSearch:      copts.dnsSearch.GetAllOrEmpty(),
		DNSOptions:     copts.dnsOptions.GetAllOrEmpty(),
		ExtraHosts:     copts.extraHosts.GetAll(),
		VolumesFrom:    copts.volumesFrom.GetAll(),
		NetworkMode:    container.NetworkMode(copts.netMode),
		IpcMode:        ipcMode,
		PidMode:        pidMode,
		UTSMode:        utsMode,
		UsernsMode:     usernsMode,
		CapAdd:         strslice.StrSlice(copts.capAdd.GetAll()),
		CapDrop:        strslice.StrSlice(copts.capDrop.GetAll()),
		GroupAdd:       copts.groupAdd.GetAll(),
		RestartPolicy:  restartPolicy,
		SecurityOpt:    securityOpts,
		StorageOpt:     storageOpts,
		ReadonlyRootfs: copts.readonlyRootfs,
		LogConfig:      container.LogConfig{Type: copts.loggingDriver, Config: loggingOpts},
		VolumeDriver:   copts.volumeDriver,
		Isolation:      container.Isolation(copts.isolation),
		ShmSize:        shmSize,
		Resources:      resources,
		Tmpfs:          tmpfs,
		Sysctls:        copts.sysctls.GetAll(),
		Runtime:        copts.runtime,
	}

	if copts.autoRemove && !hostConfig.RestartPolicy.IsNone() {
		return nil, nil, nil, fmt.Errorf("Conflicting options: --restart and --rm")
	}

	// only set this value if the user provided the flag, else it should default to nil
	if flags.Changed("init") {
		hostConfig.Init = &copts.init
	}

	// When allocating stdin in attached mode, close stdin at client disconnect
	if config.OpenStdin && config.AttachStdin {
		config.StdinOnce = true
	}

	networkingConfig := &networktypes.NetworkingConfig{
		EndpointsConfig: make(map[string]*networktypes.EndpointSettings),
	}

	if copts.ipv4Address != "" || copts.ipv6Address != "" || copts.linkLocalIPs.Len() > 0 {
		epConfig := &networktypes.EndpointSettings{}
		networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)] = epConfig

		epConfig.IPAMConfig = &networktypes.EndpointIPAMConfig{
			IPv4Address: copts.ipv4Address,
			IPv6Address: copts.ipv6Address,
		}

		if copts.linkLocalIPs.Len() > 0 {
			epConfig.IPAMConfig.LinkLocalIPs = make([]string, copts.linkLocalIPs.Len())
			copy(epConfig.IPAMConfig.LinkLocalIPs, copts.linkLocalIPs.GetAll())
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

	if copts.aliases.Len() > 0 {
		epConfig := networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)]
		if epConfig == nil {
			epConfig = &networktypes.EndpointSettings{}
		}
		epConfig.Aliases = make([]string, copts.aliases.Len())
		copy(epConfig.Aliases, copts.aliases.GetAll())
		networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)] = epConfig
	}

	return config, hostConfig, networkingConfig, nil
}

func parseLoggingOpts(loggingDriver string, loggingOpts []string) (map[string]string, error) {
	loggingOptsMap := runconfigopts.ConvertKVStringsToMap(loggingOpts)
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
			if strings.Contains(opt, ":") {
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
			return nil, fmt.Errorf("invalid storage option")
		}
	}
	return m, nil
}

// parseDevice parses a device mapping string to a container.DeviceMapping struct
func parseDevice(device string) (container.DeviceMapping, error) {
	src := ""
	dst := ""
	permissions := "rwm"
	arr := strings.Split(device, ":")
	switch len(arr) {
	case 3:
		permissions = arr[2]
		fallthrough
	case 2:
		if validDeviceMode(arr[1]) {
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

// validDeviceMode checks if the mode for device is valid or not.
// Valid mode is a composition of r (read), w (write), and m (mknod).
func validDeviceMode(mode string) bool {
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

// validateDevice validates a path for devices
// It will make sure 'val' is in the form:
//    [host-dir:]container-path[:mode]
// It also validates the device mode.
func validateDevice(val string) (string, error) {
	return validatePath(val, validDeviceMode)
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

// validateAttach validates that the specified string is a valid attach option.
func validateAttach(val string) (string, error) {
	s := strings.ToLower(val)
	for _, str := range []string{"stdin", "stdout", "stderr"} {
		if s == str {
			return s, nil
		}
	}
	return val, fmt.Errorf("valid streams are STDIN, STDOUT and STDERR")
}
