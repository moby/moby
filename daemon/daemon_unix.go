// +build linux freebsd

package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/daemon/graphdriver"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
	"github.com/docker/libnetwork"
	nwconfig "github.com/docker/libnetwork/config"
	"github.com/docker/libnetwork/drivers/bridge"
	"github.com/docker/libnetwork/ipamutils"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/types"
	"github.com/opencontainers/runc/libcontainer/label"
	"github.com/vishvananda/netlink"
)

const (
	// See https://git.kernel.org/cgit/linux/kernel/git/tip/tip.git/tree/kernel/sched/sched.h?id=8cd9234c64c584432f6992fe944ca9e46ca8ea76#n269
	linuxMinCPUShares = 2
	linuxMaxCPUShares = 262144
	platformSupported = true
)

func parseSecurityOpt(container *Container, config *runconfig.HostConfig) error {
	var (
		labelOpts []string
		err       error
	)

	for _, opt := range config.SecurityOpt {
		con := strings.SplitN(opt, ":", 2)
		if len(con) == 1 {
			return fmt.Errorf("Invalid --security-opt: %q", opt)
		}
		switch con[0] {
		case "label":
			labelOpts = append(labelOpts, con[1])
		case "apparmor":
			container.AppArmorProfile = con[1]
		default:
			return fmt.Errorf("Invalid --security-opt: %q", opt)
		}
	}

	container.ProcessLabel, container.MountLabel, err = label.InitLabels(labelOpts)
	return err
}

func checkKernelVersion(k, major, minor int) bool {
	if v, err := kernel.GetKernelVersion(); err != nil {
		logrus.Warnf("%s", err)
	} else {
		if kernel.CompareKernelVersion(*v, kernel.VersionInfo{Kernel: k, Major: major, Minor: minor}) < 0 {
			return false
		}
	}
	return true
}

func checkKernel() error {
	// Check for unsupported kernel versions
	// FIXME: it would be cleaner to not test for specific versions, but rather
	// test for specific functionalities.
	// Unfortunately we can't test for the feature "does not cause a kernel panic"
	// without actually causing a kernel panic, so we need this workaround until
	// the circumstances of pre-3.10 crashes are clearer.
	// For details see https://github.com/docker/docker/issues/407
	if !checkKernelVersion(3, 10, 0) {
		v, _ := kernel.GetKernelVersion()
		if os.Getenv("DOCKER_NOWARN_KERNEL_VERSION") == "" {
			logrus.Warnf("Your Linux kernel version %s can be unstable running docker. Please upgrade your kernel to 3.10.0.", v.String())
		}
	}
	return nil
}

// adaptContainerSettings is called during container creation to modify any
// settings necessary in the HostConfig structure.
func (daemon *Daemon) adaptContainerSettings(hostConfig *runconfig.HostConfig, adjustCPUShares bool) {
	if hostConfig == nil {
		return
	}

	if adjustCPUShares && hostConfig.CPUShares > 0 {
		// Handle unsupported CPUShares
		if hostConfig.CPUShares < linuxMinCPUShares {
			logrus.Warnf("Changing requested CPUShares of %d to minimum allowed of %d", hostConfig.CPUShares, linuxMinCPUShares)
			hostConfig.CPUShares = linuxMinCPUShares
		} else if hostConfig.CPUShares > linuxMaxCPUShares {
			logrus.Warnf("Changing requested CPUShares of %d to maximum allowed of %d", hostConfig.CPUShares, linuxMaxCPUShares)
			hostConfig.CPUShares = linuxMaxCPUShares
		}
	}
	if hostConfig.Memory > 0 && hostConfig.MemorySwap == 0 {
		// By default, MemorySwap is set to twice the size of Memory.
		hostConfig.MemorySwap = hostConfig.Memory * 2
	}
}

// verifyPlatformContainerSettings performs platform-specific validation of the
// hostconfig and config structures.
func verifyPlatformContainerSettings(daemon *Daemon, hostConfig *runconfig.HostConfig, config *runconfig.Config) ([]string, error) {
	warnings := []string{}
	sysInfo := sysinfo.New(true)

	warnings, err := daemon.verifyExperimentalContainerSettings(hostConfig, config)
	if err != nil {
		return warnings, err
	}

	if hostConfig.LxcConf.Len() > 0 && !strings.Contains(daemon.ExecutionDriver().Name(), "lxc") {
		return warnings, fmt.Errorf("Cannot use --lxc-conf with execdriver: %s", daemon.ExecutionDriver().Name())
	}

	// memory subsystem checks and adjustments
	if hostConfig.Memory != 0 && hostConfig.Memory < 4194304 {
		return warnings, fmt.Errorf("Minimum memory limit allowed is 4MB")
	}
	if hostConfig.Memory > 0 && !sysInfo.MemoryLimit {
		warnings = append(warnings, "Your kernel does not support memory limit capabilities. Limitation discarded.")
		logrus.Warnf("Your kernel does not support memory limit capabilities. Limitation discarded.")
		hostConfig.Memory = 0
		hostConfig.MemorySwap = -1
	}
	if hostConfig.Memory > 0 && hostConfig.MemorySwap != -1 && !sysInfo.SwapLimit {
		warnings = append(warnings, "Your kernel does not support swap limit capabilities, memory limited without swap.")
		logrus.Warnf("Your kernel does not support swap limit capabilities, memory limited without swap.")
		hostConfig.MemorySwap = -1
	}
	if hostConfig.Memory > 0 && hostConfig.MemorySwap > 0 && hostConfig.MemorySwap < hostConfig.Memory {
		return warnings, fmt.Errorf("Minimum memoryswap limit should be larger than memory limit, see usage.")
	}
	if hostConfig.Memory == 0 && hostConfig.MemorySwap > 0 {
		return warnings, fmt.Errorf("You should always set the Memory limit when using Memoryswap limit, see usage.")
	}
	if hostConfig.MemorySwappiness != nil && *hostConfig.MemorySwappiness != -1 && !sysInfo.MemorySwappiness {
		warnings = append(warnings, "Your kernel does not support memory swappiness capabilities, memory swappiness discarded.")
		logrus.Warnf("Your kernel does not support memory swappiness capabilities, memory swappiness discarded.")
		hostConfig.MemorySwappiness = nil
	}
	if hostConfig.MemorySwappiness != nil {
		swappiness := *hostConfig.MemorySwappiness
		if swappiness < -1 || swappiness > 100 {
			return warnings, fmt.Errorf("Invalid value: %v, valid memory swappiness range is 0-100.", swappiness)
		}
	}
	if hostConfig.MemoryReservation > 0 && !sysInfo.MemoryReservation {
		warnings = append(warnings, "Your kernel does not support memory soft limit capabilities. Limitation discarded.")
		logrus.Warnf("Your kernel does not support memory soft limit capabilities. Limitation discarded.")
		hostConfig.MemoryReservation = 0
	}
	if hostConfig.Memory > 0 && hostConfig.MemoryReservation > 0 && hostConfig.Memory < hostConfig.MemoryReservation {
		return warnings, fmt.Errorf("Minimum memory limit should be larger than memory reservation limit, see usage.")
	}
	if hostConfig.KernelMemory > 0 && !sysInfo.KernelMemory {
		warnings = append(warnings, "Your kernel does not support kernel memory limit capabilities. Limitation discarded.")
		logrus.Warnf("Your kernel does not support kernel memory limit capabilities. Limitation discarded.")
		hostConfig.KernelMemory = 0
	}
	if hostConfig.KernelMemory > 0 && !checkKernelVersion(4, 0, 0) {
		warnings = append(warnings, "You specified a kernel memory limit on a kernel older than 4.0. Kernel memory limits are experimental on older kernels, it won't work as expected and can cause your system to be unstable.")
		logrus.Warnf("You specified a kernel memory limit on a kernel older than 4.0. Kernel memory limits are experimental on older kernels, it won't work as expected and can cause your system to be unstable.")
	}
	if hostConfig.CPUShares > 0 && !sysInfo.CPUShares {
		warnings = append(warnings, "Your kernel does not support CPU shares. Shares discarded.")
		logrus.Warnf("Your kernel does not support CPU shares. Shares discarded.")
		hostConfig.CPUShares = 0
	}
	if hostConfig.CPUPeriod > 0 && !sysInfo.CPUCfsPeriod {
		warnings = append(warnings, "Your kernel does not support CPU cfs period. Period discarded.")
		logrus.Warnf("Your kernel does not support CPU cfs period. Period discarded.")
		hostConfig.CPUPeriod = 0
	}
	if hostConfig.CPUQuota > 0 && !sysInfo.CPUCfsQuota {
		warnings = append(warnings, "Your kernel does not support CPU cfs quota. Quota discarded.")
		logrus.Warnf("Your kernel does not support CPU cfs quota. Quota discarded.")
		hostConfig.CPUQuota = 0
	}
	if (hostConfig.CpusetCpus != "" || hostConfig.CpusetMems != "") && !sysInfo.Cpuset {
		warnings = append(warnings, "Your kernel does not support cpuset. Cpuset discarded.")
		logrus.Warnf("Your kernel does not support cpuset. Cpuset discarded.")
		hostConfig.CpusetCpus = ""
		hostConfig.CpusetMems = ""
	}
	cpusAvailable, err := sysInfo.IsCpusetCpusAvailable(hostConfig.CpusetCpus)
	if err != nil {
		return warnings, derr.ErrorCodeInvalidCpusetCpus.WithArgs(hostConfig.CpusetCpus)
	}
	if !cpusAvailable {
		return warnings, derr.ErrorCodeNotAvailableCpusetCpus.WithArgs(hostConfig.CpusetCpus, sysInfo.Cpus)
	}
	memsAvailable, err := sysInfo.IsCpusetMemsAvailable(hostConfig.CpusetMems)
	if err != nil {
		return warnings, derr.ErrorCodeInvalidCpusetMems.WithArgs(hostConfig.CpusetMems)
	}
	if !memsAvailable {
		return warnings, derr.ErrorCodeNotAvailableCpusetMems.WithArgs(hostConfig.CpusetMems, sysInfo.Mems)
	}
	if hostConfig.BlkioWeight > 0 && !sysInfo.BlkioWeight {
		warnings = append(warnings, "Your kernel does not support Block I/O weight. Weight discarded.")
		logrus.Warnf("Your kernel does not support Block I/O weight. Weight discarded.")
		hostConfig.BlkioWeight = 0
	}
	if hostConfig.BlkioWeight > 0 && (hostConfig.BlkioWeight < 10 || hostConfig.BlkioWeight > 1000) {
		return warnings, fmt.Errorf("Range of blkio weight is from 10 to 1000.")
	}
	if hostConfig.OomKillDisable && !sysInfo.OomKillDisable {
		hostConfig.OomKillDisable = false
		return warnings, fmt.Errorf("Your kernel does not support oom kill disable.")
	}

	if sysInfo.IPv4ForwardingDisabled {
		warnings = append(warnings, "IPv4 forwarding is disabled. Networking will not work.")
		logrus.Warnf("IPv4 forwarding is disabled. Networking will not work")
	}
	return warnings, nil
}

// checkConfigOptions checks for mutually incompatible config options
func checkConfigOptions(config *Config) error {
	// Check for mutually incompatible config options
	if config.Bridge.Iface != "" && config.Bridge.IP != "" {
		return fmt.Errorf("You specified -b & --bip, mutually exclusive options. Please specify only one.")
	}
	if !config.Bridge.EnableIPTables && !config.Bridge.InterContainerCommunication {
		return fmt.Errorf("You specified --iptables=false with --icc=false. ICC uses iptables to function. Please set --icc or --iptables to true.")
	}
	if !config.Bridge.EnableIPTables && config.Bridge.EnableIPMasq {
		config.Bridge.EnableIPMasq = false
	}
	return nil
}

// checkSystem validates platform-specific requirements
func checkSystem() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("The Docker daemon needs to be run as root")
	}
	return checkKernel()
}

// configureKernelSecuritySupport configures and validate security support for the kernel
func configureKernelSecuritySupport(config *Config, driverName string) error {
	if config.EnableSelinuxSupport {
		if selinuxEnabled() {
			// As Docker on either btrfs or overlayFS and SELinux are incompatible at present, error on both being enabled
			if driverName == "btrfs" || driverName == "overlay" {
				return fmt.Errorf("SELinux is not supported with the %s graph driver", driverName)
			}
			logrus.Debug("SELinux enabled successfully")
		} else {
			logrus.Warn("Docker could not enable SELinux on the host system")
		}
	} else {
		selinuxSetDisabled()
	}
	return nil
}

// MigrateIfDownlevel is a wrapper for AUFS migration for downlevel
func migrateIfDownlevel(driver graphdriver.Driver, root string) error {
	return migrateIfAufs(driver, root)
}

func configureSysInit(config *Config, rootUID, rootGID int) (string, error) {
	localCopy := filepath.Join(config.Root, "init", fmt.Sprintf("dockerinit-%s", dockerversion.VERSION))
	sysInitPath := utils.DockerInitPath(localCopy)
	if sysInitPath == "" {
		return "", fmt.Errorf("Could not locate dockerinit: This usually means docker was built incorrectly. See https://docs.docker.com/project/set-up-dev-env/ for official build instructions.")
	}

	if sysInitPath != localCopy {
		// When we find a suitable dockerinit binary (even if it's our local binary), we copy it into config.Root at localCopy for future use (so that the original can go away without that being a problem, for example during a package upgrade).
		if err := idtools.MkdirAs(filepath.Dir(localCopy), 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
			return "", err
		}
		if _, err := fileutils.CopyFile(sysInitPath, localCopy); err != nil {
			return "", err
		}
		if err := os.Chmod(localCopy, 0700); err != nil {
			return "", err
		}
		sysInitPath = localCopy
	}
	return sysInitPath, nil
}

func isBridgeNetworkDisabled(config *Config) bool {
	return config.Bridge.Iface == disableNetworkBridge
}

func (daemon *Daemon) networkOptions(dconfig *Config) ([]nwconfig.Option, error) {
	options := []nwconfig.Option{}
	if dconfig == nil {
		return options, nil
	}

	options = append(options, nwconfig.OptionDataDir(dconfig.Root))

	if strings.TrimSpace(dconfig.DefaultNetwork) != "" {
		dn := strings.Split(dconfig.DefaultNetwork, ":")
		if len(dn) < 2 {
			return nil, fmt.Errorf("default network daemon config must be of the form NETWORKDRIVER:NETWORKNAME")
		}
		options = append(options, nwconfig.OptionDefaultDriver(dn[0]))
		options = append(options, nwconfig.OptionDefaultNetwork(strings.Join(dn[1:], ":")))
	} else {
		dd := runconfig.DefaultDaemonNetworkMode()
		dn := runconfig.DefaultDaemonNetworkMode().NetworkName()
		options = append(options, nwconfig.OptionDefaultDriver(string(dd)))
		options = append(options, nwconfig.OptionDefaultNetwork(dn))
	}

	if strings.TrimSpace(dconfig.ClusterStore) != "" {
		kv := strings.Split(dconfig.ClusterStore, "://")
		if len(kv) < 2 {
			return nil, fmt.Errorf("kv store daemon config must be of the form KV-PROVIDER://KV-URL")
		}
		options = append(options, nwconfig.OptionKVProvider(kv[0]))
		options = append(options, nwconfig.OptionKVProviderURL(strings.Join(kv[1:], "://")))
	}
	if len(dconfig.ClusterOpts) > 0 {
		options = append(options, nwconfig.OptionKVOpts(dconfig.ClusterOpts))
	}

	if daemon.discoveryWatcher != nil {
		options = append(options, nwconfig.OptionDiscoveryWatcher(daemon.discoveryWatcher))
	}

	if dconfig.ClusterAdvertise != "" {
		options = append(options, nwconfig.OptionDiscoveryAddress(dconfig.ClusterAdvertise))
	}

	options = append(options, nwconfig.OptionLabels(dconfig.Labels))
	options = append(options, driverOptions(dconfig)...)
	return options, nil
}

func (daemon *Daemon) initNetworkController(config *Config) (libnetwork.NetworkController, error) {
	netOptions, err := daemon.networkOptions(config)
	if err != nil {
		return nil, err
	}

	controller, err := libnetwork.New(netOptions...)
	if err != nil {
		return nil, fmt.Errorf("error obtaining controller instance: %v", err)
	}

	// Initialize default network on "null"
	if _, err := controller.NewNetwork("null", "none", libnetwork.NetworkOptionPersist(false)); err != nil {
		return nil, fmt.Errorf("Error creating default \"null\" network: %v", err)
	}

	// Initialize default network on "host"
	if _, err := controller.NewNetwork("host", "host", libnetwork.NetworkOptionPersist(false)); err != nil {
		return nil, fmt.Errorf("Error creating default \"host\" network: %v", err)
	}

	if !config.DisableBridge {
		// Initialize default driver "bridge"
		if err := initBridgeDriver(controller, config); err != nil {
			return nil, err
		}
	}

	return controller, nil
}

func driverOptions(config *Config) []nwconfig.Option {
	bridgeConfig := options.Generic{
		"EnableIPForwarding":  config.Bridge.EnableIPForward,
		"EnableIPTables":      config.Bridge.EnableIPTables,
		"EnableUserlandProxy": config.Bridge.EnableUserlandProxy}
	bridgeOption := options.Generic{netlabel.GenericData: bridgeConfig}

	dOptions := []nwconfig.Option{}
	dOptions = append(dOptions, nwconfig.OptionDriverConfig("bridge", bridgeOption))
	return dOptions
}

func initBridgeDriver(controller libnetwork.NetworkController, config *Config) error {
	if n, err := controller.NetworkByName("bridge"); err == nil {
		if err = n.Delete(); err != nil {
			return fmt.Errorf("could not delete the default bridge network: %v", err)
		}
	}

	bridgeName := bridge.DefaultBridgeName
	if config.Bridge.Iface != "" {
		bridgeName = config.Bridge.Iface
	}
	netOption := map[string]string{
		bridge.BridgeName:         bridgeName,
		bridge.DefaultBridge:      strconv.FormatBool(true),
		netlabel.DriverMTU:        strconv.Itoa(config.Mtu),
		bridge.EnableIPMasquerade: strconv.FormatBool(config.Bridge.EnableIPMasq),
		bridge.EnableICC:          strconv.FormatBool(config.Bridge.InterContainerCommunication),
	}

	// --ip processing
	if config.Bridge.DefaultIP != nil {
		netOption[bridge.DefaultBindingIP] = config.Bridge.DefaultIP.String()
	}

	ipamV4Conf := libnetwork.IpamConf{}

	ipamV4Conf.AuxAddresses = make(map[string]string)

	if nw, _, err := ipamutils.ElectInterfaceAddresses(bridgeName); err == nil {
		ipamV4Conf.PreferredPool = nw.String()
		hip, _ := types.GetHostPartIP(nw.IP, nw.Mask)
		if hip.IsGlobalUnicast() {
			ipamV4Conf.Gateway = nw.IP.String()
		}
	}

	if config.Bridge.IP != "" {
		ipamV4Conf.PreferredPool = config.Bridge.IP
		ip, _, err := net.ParseCIDR(config.Bridge.IP)
		if err != nil {
			return err
		}
		ipamV4Conf.Gateway = ip.String()
	} else if bridgeName == bridge.DefaultBridgeName && ipamV4Conf.PreferredPool != "" {
		logrus.Infof("Default bridge (%s) is assigned with an IP address %s. Daemon option --bip can be used to set a preferred IP address", bridgeName, ipamV4Conf.PreferredPool)
	}

	if config.Bridge.FixedCIDR != "" {
		_, fCIDR, err := net.ParseCIDR(config.Bridge.FixedCIDR)
		if err != nil {
			return err
		}

		ipamV4Conf.SubPool = fCIDR.String()
	}

	if config.Bridge.DefaultGatewayIPv4 != nil {
		ipamV4Conf.AuxAddresses["DefaultGatewayIPv4"] = config.Bridge.DefaultGatewayIPv4.String()
	}

	var ipamV6Conf *libnetwork.IpamConf
	if config.Bridge.FixedCIDRv6 != "" {
		_, fCIDRv6, err := net.ParseCIDR(config.Bridge.FixedCIDRv6)
		if err != nil {
			return err
		}
		if ipamV6Conf == nil {
			ipamV6Conf = &libnetwork.IpamConf{}
		}
		ipamV6Conf.PreferredPool = fCIDRv6.String()
	}

	if config.Bridge.DefaultGatewayIPv6 != nil {
		if ipamV6Conf == nil {
			ipamV6Conf = &libnetwork.IpamConf{}
		}
		ipamV6Conf.AuxAddresses["DefaultGatewayIPv6"] = config.Bridge.DefaultGatewayIPv6.String()
	}

	v4Conf := []*libnetwork.IpamConf{&ipamV4Conf}
	v6Conf := []*libnetwork.IpamConf{}
	if ipamV6Conf != nil {
		v6Conf = append(v6Conf, ipamV6Conf)
	}
	// Initialize default network on "bridge" with the same name
	_, err := controller.NewNetwork("bridge", "bridge",
		libnetwork.NetworkOptionGeneric(options.Generic{
			netlabel.GenericData: netOption,
			netlabel.EnableIPv6:  config.Bridge.EnableIPv6,
		}),
		libnetwork.NetworkOptionIpam("default", "", v4Conf, v6Conf))
	if err != nil {
		return fmt.Errorf("Error creating default \"bridge\" network: %v", err)
	}
	return nil
}

// setupInitLayer populates a directory with mountpoints suitable
// for bind-mounting dockerinit into the container. The mountpoint is simply an
// empty file at /.dockerinit
//
// This extra layer is used by all containers as the top-most ro layer. It protects
// the container from unwanted side-effects on the rw layer.
func setupInitLayer(initLayer string, rootUID, rootGID int) error {
	for pth, typ := range map[string]string{
		"/dev/pts":         "dir",
		"/dev/shm":         "dir",
		"/proc":            "dir",
		"/sys":             "dir",
		"/.dockerinit":     "file",
		"/.dockerenv":      "file",
		"/etc/resolv.conf": "file",
		"/etc/hosts":       "file",
		"/etc/hostname":    "file",
		"/dev/console":     "file",
		"/etc/mtab":        "/proc/mounts",
	} {
		parts := strings.Split(pth, "/")
		prev := "/"
		for _, p := range parts[1:] {
			prev = filepath.Join(prev, p)
			syscall.Unlink(filepath.Join(initLayer, prev))
		}

		if _, err := os.Stat(filepath.Join(initLayer, pth)); err != nil {
			if os.IsNotExist(err) {
				if err := idtools.MkdirAllAs(filepath.Join(initLayer, filepath.Dir(pth)), 0755, rootUID, rootGID); err != nil {
					return err
				}
				switch typ {
				case "dir":
					if err := idtools.MkdirAllAs(filepath.Join(initLayer, pth), 0755, rootUID, rootGID); err != nil {
						return err
					}
				case "file":
					f, err := os.OpenFile(filepath.Join(initLayer, pth), os.O_CREATE, 0755)
					if err != nil {
						return err
					}
					f.Close()
					f.Chown(rootUID, rootGID)
				default:
					if err := os.Symlink(typ, filepath.Join(initLayer, pth)); err != nil {
						return err
					}
				}
			} else {
				return err
			}
		}
	}

	// Layer is ready to use, if it wasn't before.
	return nil
}

// registerLinks writes the links to a file.
func (daemon *Daemon) registerLinks(container *Container, hostConfig *runconfig.HostConfig) error {
	if hostConfig == nil || hostConfig.Links == nil {
		return nil
	}

	for _, l := range hostConfig.Links {
		name, alias, err := parsers.ParseLink(l)
		if err != nil {
			return err
		}
		child, err := daemon.Get(name)
		if err != nil {
			//An error from daemon.Get() means this name could not be found
			return fmt.Errorf("Could not get container for %s", name)
		}
		for child.hostConfig.NetworkMode.IsContainer() {
			parts := strings.SplitN(string(child.hostConfig.NetworkMode), ":", 2)
			child, err = daemon.Get(parts[1])
			if err != nil {
				return fmt.Errorf("Could not get container for %s", parts[1])
			}
		}
		if child.hostConfig.NetworkMode.IsHost() {
			return runconfig.ErrConflictHostNetworkAndLinks
		}
		if err := daemon.registerLink(container, child, alias); err != nil {
			return err
		}
	}

	// After we load all the links into the daemon
	// set them to nil on the hostconfig
	hostConfig.Links = nil
	if err := container.writeHostConfig(); err != nil {
		return err
	}

	return nil
}

func (daemon *Daemon) newBaseContainer(id string) Container {
	return Container{
		CommonContainer: CommonContainer{
			ID:           id,
			State:        NewState(),
			execCommands: newExecStore(),
			root:         daemon.containerRoot(id),
		},
		MountPoints: make(map[string]*mountPoint),
		Volumes:     make(map[string]string),
		VolumesRW:   make(map[string]bool),
	}
}

// getDefaultRouteMtu returns the MTU for the default route's interface.
func getDefaultRouteMtu() (int, error) {
	routes, err := netlink.RouteList(nil, 0)
	if err != nil {
		return 0, err
	}
	for _, r := range routes {
		// a nil Dst means that this is the default route.
		if r.Dst == nil {
			i, err := net.InterfaceByIndex(r.LinkIndex)
			if err != nil {
				continue
			}
			return i.MTU, nil
		}
	}
	return 0, errNoDefaultRoute
}
