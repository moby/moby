// +build linux

package runconfig

import (
	"fmt"
	"strings"

	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/ulimit"
)

var (
	ErrConflictNetworkHosts = fmt.Errorf("Conflicting options: --add-host and the network mode (--net).")
)

func parsePlatformSpecific(cmd *flag.FlagSet, hostconfig *HostConfig, netMode NetworkMode) error {

	// TODO Windows: For the daemon to compile on Windows, references to all
	// of these fields needs to be factored out. However, waiting for other
	// PRs to be merged before this can be done as those files already have
	// other changes pending. For example GH12884

	var (
		ulimits          = make(map[string]*ulimit.Ulimit)
		flUlimits        = opts.NewUlimitOpt(ulimits)
		flLxcOpts        = opts.NewListOpts(nil)
		flDevices        = opts.NewListOpts(opts.ValidatePath)
		flIpcMode        = cmd.String([]string{"-ipc"}, "", "IPC namespace to use")
		flPidMode        = cmd.String([]string{"-pid"}, "", "PID namespace to use")
		flBlkioWeight    = cmd.Int64([]string{"-blkio-weight"}, 0, "Block IO (relative weight), between 10 and 1000")
		flOomKillDisable = cmd.Bool([]string{"-oom-kill-disable"}, false, "Disable OOM Killer")
		flPrivileged     = cmd.Bool([]string{"#privileged", "-privileged"}, false, "Give extended privileges to this container")
		flExtraHosts     = opts.NewListOpts(opts.ValidateExtraHost)
		flVolumesFrom    = opts.NewListOpts(nil)
		flCapAdd         = opts.NewListOpts(nil)
		flCapDrop        = opts.NewListOpts(nil)
		flSecurityOpt    = opts.NewListOpts(nil)
		flReadonlyRootfs = cmd.Bool([]string{"-read-only"}, false, "Mount the container's root filesystem as read only")
		flCgroupParent   = cmd.String([]string{"-cgroup-parent"}, "", "Optional parent cgroup for the container")
	)

	cmd.Var(flUlimits, []string{"-ulimit"}, "Ulimit options")
	cmd.Var(&flLxcOpts, []string{"#lxc-conf", "-lxc-conf"}, "Add custom lxc options")
	cmd.Var(&flDevices, []string{"-device"}, "Add a host device to the container")
	cmd.Var(&flExtraHosts, []string{"-add-host"}, "Add a custom host-to-IP mapping (host:ip)")
	cmd.Var(&flVolumesFrom, []string{"#volumes-from", "-volumes-from"}, "Mount volumes from the specified container(s)")
	cmd.Var(&flCapAdd, []string{"-cap-add"}, "Add Linux capabilities")
	cmd.Var(&flCapDrop, []string{"-cap-drop"}, "Drop Linux capabilities")
	cmd.Var(&flSecurityOpt, []string{"-security-opt"}, "Security Options")

	lc, err := parseKeyValueOpts(flLxcOpts)
	if err != nil {
		return err
	}
	lxcConf := NewLxcConfig(lc)

	// parse device mappings
	deviceMappings := []DeviceMapping{}
	for _, device := range flDevices.GetAll() {
		deviceMapping, err := ParseDevice(device)
		if err != nil {
			return err
		}
		deviceMappings = append(deviceMappings, deviceMapping)
	}

	ipcMode := IpcMode(*flIpcMode)
	if !ipcMode.Valid() {
		return fmt.Errorf("--ipc: invalid IPC mode")
	}

	pidMode := PidMode(*flPidMode)
	if !pidMode.Valid() {
		return fmt.Errorf("--pid: invalid PID mode")
	}

	if (netMode.IsContainer() || netMode.IsHost()) && flExtraHosts.Len() > 0 {
		return ErrConflictNetworkHosts
	}

	hostconfig.Ulimits = flUlimits.GetList()
	hostconfig.LxcConf = lxcConf
	hostconfig.Devices = deviceMappings
	hostconfig.IpcMode = ipcMode
	hostconfig.PidMode = pidMode
	hostconfig.BlkioWeight = *flBlkioWeight
	hostconfig.OomKillDisable = *flOomKillDisable
	hostconfig.Privileged = *flPrivileged
	hostconfig.ExtraHosts = flExtraHosts.GetAll()
	hostconfig.VolumesFrom = flVolumesFrom.GetAll()
	hostconfig.CapAdd = flCapAdd.GetAll()
	hostconfig.CapDrop = flCapDrop.GetAll()
	hostconfig.SecurityOpt = flSecurityOpt.GetAll()
	hostconfig.ReadonlyRootfs = *flReadonlyRootfs
	hostconfig.CgroupParent = *flCgroupParent

	return nil
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
