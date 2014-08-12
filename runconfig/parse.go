package runconfig

import (
	"fmt"
	"io/ioutil"
	"path"
	"strconv"
	"strings"

	"github.com/docker/docker/nat"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/units"
	"github.com/docker/docker/utils"
)

var (
	ErrInvalidWorkingDirectory            = fmt.Errorf("The working directory is invalid. It needs to be an absolute path.")
	ErrConflictAttachDetach               = fmt.Errorf("Conflicting options: -a and -d")
	ErrConflictDetachAutoRemove           = fmt.Errorf("Conflicting options: --rm and -d")
	ErrConflictNetworkHostname            = fmt.Errorf("Conflicting options: -h and the network mode (--net)")
	ErrConflictHostNetworkAndLinks        = fmt.Errorf("Conflicting options: --net=host can't be used with links. This would result in undefined behavior.")
	ErrConflictRestartPolicyAndAutoRemove = fmt.Errorf("Conflicting options: --restart and --rm")
)

//FIXME Only used in tests
func Parse(args []string, sysInfo *sysinfo.SysInfo) (*Config, *HostConfig, *flag.FlagSet, error) {
	cmd := flag.NewFlagSet("run", flag.ContinueOnError)
	cmd.SetOutput(ioutil.Discard)
	cmd.Usage = nil
	return parseRun(cmd, args, sysInfo)
}

// FIXME: this maps the legacy commands.go code. It should be merged with Parse to only expose a single parse function.
func ParseSubcommand(cmd *flag.FlagSet, args []string, sysInfo *sysinfo.SysInfo) (*Config, *HostConfig, *flag.FlagSet, error) {
	return parseRun(cmd, args, sysInfo)
}

func parseRun(cmd *flag.FlagSet, args []string, sysInfo *sysinfo.SysInfo) (*Config, *HostConfig, *flag.FlagSet, error) {
	var (
		flAttach      = make(map[string]struct{})
		flVolumes     = make(map[string]struct{})
		flLinks       []string
		flEnv         []string
		flDevices     []string
		flPublish     []string
		flExpose      []string
		flDns         []string
		flDnsSearch   []string
		flVolumesFrom []string
		flLxcOpts     []string
		flEnvFile     []string
		flCapAdd      []string
		flCapDrop     []string

		// FIXME: switch all parsed values to regular types, then attach them with flag.xxxVar, for consistency
		flAutoRemove      = cmd.Bool([]string{"#rm", "-rm"}, false, "Automatically remove the container when it exits (incompatible with -d)")
		flDetach          = cmd.Bool([]string{"d", "-detach"}, false, "Detached mode: run container in the background and print new container ID")
		flNetwork         = cmd.Bool([]string{"#n", "#-networking"}, true, "Enable networking for this container")
		flPrivileged      = cmd.Bool([]string{"#privileged", "-privileged"}, false, "Give extended privileges to this container")
		flPublishAll      = cmd.Bool([]string{"P", "-publish-all"}, false, "Publish all exposed ports to the host interfaces")
		flStdin           = cmd.Bool([]string{"i", "-interactive"}, false, "Keep STDIN open even if not attached")
		flTty             = cmd.Bool([]string{"t", "-tty"}, false, "Allocate a pseudo-TTY")
		flContainerIDFile = cmd.String([]string{"#cidfile", "-cidfile"}, "", "Write the container ID to the file")
		flEntrypoint      = cmd.String([]string{"#entrypoint", "-entrypoint"}, "", "Overwrite the default ENTRYPOINT of the image")
		flHostname        = cmd.String([]string{"h", "-hostname"}, "", "Container host name")
		flMemoryString    = cmd.String([]string{"m", "-memory"}, "", "Memory limit (format: <number><optional unit>, where unit = b, k, m or g)")
		flUser            = cmd.String([]string{"u", "-user"}, "", "Username or UID")
		flWorkingDir      = cmd.String([]string{"w", "-workdir"}, "", "Working directory inside the container")
		flCpuShares       = cmd.Int64([]string{"c", "-cpu-shares"}, 0, "CPU shares (relative weight)")
		flCpuset          = cmd.String([]string{"-cpuset"}, "", "CPUs in which to allow execution (0-3, 0,1)")
		flNetMode         = cmd.String([]string{"-net"}, "bridge", "Set the Network mode for the container\n'bridge': creates a new network stack for the container on the docker bridge\n'none': no networking for this container\n'container:<name|id>': reuses another container network stack\n'host': use the host network stack inside the container.  Note: the host mode gives the container full access to local system services such as D-bus and is therefore considered insecure.")
		flRestartPolicy   = cmd.String([]string{"-restart"}, "", "Restart policy to apply when a container exits (no, on-failure, always)")
		// For documentation purpose
		_ = cmd.Bool([]string{"#sig-proxy", "-sig-proxy"}, true, "Proxy received signals to the process (even in non-TTY mode). SIGCHLD, SIGSTOP, and SIGKILL are not proxied.")
		_ = cmd.String([]string{"#name", "-name"}, "", "Assign a name to the container")
	)

	// FIXME
	opts.StreamSetVar(&flAttach, []string{"a", "-attach"}, "Attach to STDIN, STDOUT or STDERR.")
	opts.PathPairSetVar(&flVolumes, []string{"v", "-volume"}, "Bind mount a volume (e.g., from the host: -v /host:/container, from Docker: -v /container)")
	opts.NamePairListVar(&flLinks, []string{"#link", "-link"}, "Add link to another container in the form of name:alias")
	opts.PathPairListVar(&flDevices, []string{"-device"}, "Add a host device to the container (e.g. --device=/dev/sdc:/dev/xvdc)")
	opts.EnvVar(&flEnv, []string{"e", "-env"}, "Set environment variables")
	opts.ListVar(&flEnvFile, []string{"-env-file"}, "Read in a line delimited file of environment variables")
	opts.ListVar(&flPublish, []string{"p", "-publish"}, fmt.Sprintf("Publish a container's port to the host\nformat: %s\n(use 'docker port' to see the actual mapping)", nat.PortSpecTemplateFormat))
	opts.ListVar(&flExpose, []string{"#expose", "-expose"}, "Expose a port from the container without publishing it to your host")
	opts.IPListVar(&flDns, []string{"#dns", "-dns"}, "Set custom DNS servers")
	opts.DnsSearchListVar(&flDnsSearch, []string{"-dns-search"}, "Set custom DNS search domains")
	opts.ListVar(&flVolumesFrom, []string{"#volumes-from", "-volumes-from"}, "Mount volumes from the specified container(s)")
	opts.ListVar(&flLxcOpts, []string{"#lxc-conf", "-lxc-conf"}, "(lxc exec-driver only) Add custom lxc options --lxc-conf=\"lxc.cgroup.cpuset.cpus = 0,1\"")
	opts.ListVar(&flCapAdd, []string{"-cap-add"}, "Add Linux capabilities")
	opts.ListVar(&flCapDrop, []string{"-cap-drop"}, "Drop Linux capabilities")

	if err := cmd.Parse(args); err != nil {
		return nil, nil, cmd, err
	}

	// Check if the kernel supports memory limit cgroup.
	if sysInfo != nil && *flMemoryString != "" && !sysInfo.MemoryLimit {
		*flMemoryString = ""
	}

	// Validate input params
	if *flDetach && len(flAttach) > 0 {
		return nil, nil, cmd, ErrConflictAttachDetach
	}
	if *flWorkingDir != "" && !path.IsAbs(*flWorkingDir) {
		return nil, nil, cmd, ErrInvalidWorkingDirectory
	}
	if *flDetach && *flAutoRemove {
		return nil, nil, cmd, ErrConflictDetachAutoRemove
	}

	if *flNetMode != "bridge" && *flNetMode != "none" && *flHostname != "" {
		return nil, nil, cmd, ErrConflictNetworkHostname
	}

	if *flNetMode == "host" && len(flLinks) > 0 {
		return nil, nil, cmd, ErrConflictHostNetworkAndLinks
	}

	// If neither -d or -a are set, attach to everything by default
	if len(flAttach) == 0 && !*flDetach {
		if !*flDetach {
			flAttach["stdout"] = struct{}{}
			flAttach["stderr"] = struct{}{}
			if *flStdin {
				flAttach["stdin"] = struct{}{}
			}
		}
	}

	var flMemory int64
	if *flMemoryString != "" {
		parsedMemory, err := units.RAMInBytes(*flMemoryString)
		if err != nil {
			return nil, nil, cmd, err
		}
		flMemory = parsedMemory
	}

	var binds []string
	// add any bind targets to the list of container volumes
	for bind := range flVolumes {
		if arr := strings.Split(bind, ":"); len(arr) > 1 {
			if arr[1] == "/" {
				return nil, nil, cmd, fmt.Errorf("Invalid bind mount: destination can't be '/'")
			}
			// after creating the bind mount we want to delete it from the flVolumes values because
			// we do not want bind mounts being committed to image configs
			binds = append(binds, bind)
			delete(flVolumes, bind)
		} else if bind == "/" {
			return nil, nil, cmd, fmt.Errorf("Invalid volume: path can't be '/'")
		}
	}

	var (
		parsedArgs = cmd.Args()
		runCmd     []string
		entrypoint []string
		image      string
	)
	if len(parsedArgs) >= 1 {
		image = cmd.Arg(0)
	}
	if len(parsedArgs) > 1 {
		runCmd = parsedArgs[1:]
	}
	if *flEntrypoint != "" {
		entrypoint = []string{*flEntrypoint}
	}

	lxcConf, err := parseKeyValueOpts(flLxcOpts)
	if err != nil {
		return nil, nil, cmd, err
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

	ports, portBindings, err := nat.ParsePortSpecs(flPublish)
	if err != nil {
		return nil, nil, cmd, err
	}

	// Merge in exposed ports to the map of published ports
	for _, e := range flExpose {
		if strings.Contains(e, ":") {
			return nil, nil, cmd, fmt.Errorf("Invalid port format for --expose: %s", e)
		}
		p := nat.NewPort(nat.SplitProtoPort(e))
		if _, exists := ports[p]; !exists {
			ports[p] = struct{}{}
		}
	}

	// parse device mappings
	deviceMappings := []DeviceMapping{}
	for _, device := range flDevices {
		deviceMapping, err := ParseDevice(device)
		if err != nil {
			return nil, nil, cmd, err
		}
		deviceMappings = append(deviceMappings, deviceMapping)
	}

	// collect all the environment variables for the container
	envVariables := []string{}
	for _, ef := range flEnvFile {
		parsedVars, err := opts.ParseEnvFile(ef)
		if err != nil {
			return nil, nil, cmd, err
		}
		envVariables = append(envVariables, parsedVars...)
	}
	// parse the '-e' and '--env' after, to allow override
	envVariables = append(envVariables, flEnv...)

	netMode, err := parseNetMode(*flNetMode)
	if err != nil {
		return nil, nil, cmd, fmt.Errorf("--net: invalid net mode: %v", err)
	}

	restartPolicy, err := parseRestartPolicy(*flRestartPolicy)
	if err != nil {
		return nil, nil, cmd, err
	}

	if *flAutoRemove && (restartPolicy.Name == "always" || restartPolicy.Name == "on-failure") {
		return nil, nil, cmd, ErrConflictRestartPolicyAndAutoRemove
	}

	config := &Config{
		Hostname:        hostname,
		Domainname:      domainname,
		PortSpecs:       nil, // Deprecated
		ExposedPorts:    ports,
		User:            *flUser,
		Tty:             *flTty,
		NetworkDisabled: !*flNetwork,
		OpenStdin:       *flStdin,
		Memory:          flMemory,
		CpuShares:       *flCpuShares,
		Cpuset:          *flCpuset,
		Env:             envVariables,
		Cmd:             runCmd,
		Image:           image,
		Volumes:         flVolumes,
		Entrypoint:      entrypoint,
		WorkingDir:      *flWorkingDir,
	}
	if _, ok := flAttach["stdin"]; ok {
		config.AttachStdin = true
	}
	if _, ok := flAttach["stdout"]; ok {
		config.AttachStdout = true
	}
	if _, ok := flAttach["stderr"]; ok {
		config.AttachStderr = true
	}

	hostConfig := &HostConfig{
		Binds:           binds,
		ContainerIDFile: *flContainerIDFile,
		LxcConf:         lxcConf,
		Privileged:      *flPrivileged,
		PortBindings:    portBindings,
		Links:           flLinks,
		PublishAllPorts: *flPublishAll,
		Dns:             flDns,
		DnsSearch:       flDnsSearch,
		VolumesFrom:     flVolumesFrom,
		NetworkMode:     netMode,
		Devices:         deviceMappings,
		CapAdd:          flCapAdd,
		CapDrop:         flCapDrop,
		RestartPolicy:   restartPolicy,
	}

	if sysInfo != nil && flMemory > 0 && !sysInfo.SwapLimit {
		//fmt.Fprintf(stdout, "WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.\n")
		config.MemorySwap = -1
	}

	// When allocating stdin in attached mode, close stdin at client disconnect
	if config.OpenStdin && config.AttachStdin {
		config.StdinOnce = true
	}
	return config, hostConfig, cmd, nil
}

// parseRestartPolicy returns the parsed policy or an error indicating what is incorrect
func parseRestartPolicy(policy string) (RestartPolicy, error) {
	p := RestartPolicy{}

	if policy == "" {
		return p, nil
	}

	var (
		parts = strings.Split(policy, ":")
		name  = parts[0]
	)

	switch name {
	case "always":
		p.Name = name

		if len(parts) == 2 {
			return p, fmt.Errorf("maximum restart count not valid with restart policy of \"always\"")
		}
	case "no":
		// do nothing
	case "on-failure":
		p.Name = name

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

// options will come in the format of name.key=value or name.option
func parseDriverOpts(opts []string) (map[string][]string, error) {
	out := make(map[string][]string, len(opts))
	for _, o := range opts {
		parts := strings.SplitN(o, ".", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid opt format %s", o)
		} else if strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("key cannot be empty %s", o)
		}
		values, exists := out[parts[0]]
		if !exists {
			values = []string{}
		}
		out[parts[0]] = append(values, parts[1])
	}
	return out, nil
}

func parseKeyValueOpts(opts []string) ([]utils.KeyValuePair, error) {
	out := make([]utils.KeyValuePair, len(opts))
	for i, o := range opts {
		k, v, err := parsers.ParseKeyValueOpt(o)
		if err != nil {
			return nil, err
		}
		out[i] = utils.KeyValuePair{Key: k, Value: v}
	}
	return out, nil
}

func parseNetMode(netMode string) (NetworkMode, error) {
	parts := strings.Split(netMode, ":")
	switch mode := parts[0]; mode {
	case "bridge", "none", "host":
	case "container":
		if len(parts) < 2 || parts[1] == "" {
			return "", fmt.Errorf("invalid container format container:<name|id>")
		}
	default:
		return "", fmt.Errorf("invalid --net: %s", netMode)
	}
	return NetworkMode(netMode), nil
}

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
		dst = arr[1]
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
