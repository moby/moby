package client

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/ioutils"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
	"github.com/docker/go-units"
)

// CmdInfo displays system-wide information.
//
// Usage: docker info
func (cli *DockerCli) CmdInfo(args ...string) error {
	cmd := Cli.Subcmd("info", nil, Cli.DockerCommands["info"].Description, true)
	cmd.Require(flag.Exact, 0)

	cmd.ParseFlags(args, true)

	info, err := cli.client.Info(context.Background())
	if err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "Containers: %d\n", info.Containers)
	fmt.Fprintf(cli.out, " Running: %d\n", info.ContainersRunning)
	fmt.Fprintf(cli.out, " Paused: %d\n", info.ContainersPaused)
	fmt.Fprintf(cli.out, " Stopped: %d\n", info.ContainersStopped)
	fmt.Fprintf(cli.out, "Images: %d\n", info.Images)
	ioutils.FprintfIfNotEmpty(cli.out, "Server Version: %s\n", info.ServerVersion)
	ioutils.FprintfIfNotEmpty(cli.out, "Storage Driver: %s\n", info.Driver)
	if info.DriverStatus != nil {
		for _, pair := range info.DriverStatus {
			fmt.Fprintf(cli.out, " %s: %s\n", pair[0], pair[1])

			// print a warning if devicemapper is using a loopback file
			if pair[0] == "Data loop file" {
				fmt.Fprintln(cli.err, " WARNING: Usage of loopback devices is strongly discouraged for production use. Either use `--storage-opt dm.thinpooldev` or use `--storage-opt dm.no_warn_on_loop_devices=true` to suppress this warning.")
			}
		}

	}
	if info.SystemStatus != nil {
		for _, pair := range info.SystemStatus {
			fmt.Fprintf(cli.out, "%s: %s\n", pair[0], pair[1])
		}
	}
	ioutils.FprintfIfNotEmpty(cli.out, "Execution Driver: %s\n", info.ExecutionDriver)
	ioutils.FprintfIfNotEmpty(cli.out, "Logging Driver: %s\n", info.LoggingDriver)
	ioutils.FprintfIfNotEmpty(cli.out, "Cgroup Driver: %s\n", info.CgroupDriver)

	fmt.Fprintf(cli.out, "Plugins: \n")
	fmt.Fprintf(cli.out, " Volume:")
	fmt.Fprintf(cli.out, " %s", strings.Join(info.Plugins.Volume, " "))
	fmt.Fprintf(cli.out, "\n")
	fmt.Fprintf(cli.out, " Network:")
	fmt.Fprintf(cli.out, " %s", strings.Join(info.Plugins.Network, " "))
	fmt.Fprintf(cli.out, "\n")

	if len(info.Plugins.Authorization) != 0 {
		fmt.Fprintf(cli.out, " Authorization:")
		fmt.Fprintf(cli.out, " %s", strings.Join(info.Plugins.Authorization, " "))
		fmt.Fprintf(cli.out, "\n")
	}

	ioutils.FprintfIfNotEmpty(cli.out, "Kernel Version: %s\n", info.KernelVersion)
	ioutils.FprintfIfNotEmpty(cli.out, "Operating System: %s\n", info.OperatingSystem)
	ioutils.FprintfIfNotEmpty(cli.out, "OSType: %s\n", info.OSType)
	ioutils.FprintfIfNotEmpty(cli.out, "Architecture: %s\n", info.Architecture)
	fmt.Fprintf(cli.out, "CPUs: %d\n", info.NCPU)
	fmt.Fprintf(cli.out, "Total Memory: %s\n", units.BytesSize(float64(info.MemTotal)))
	ioutils.FprintfIfNotEmpty(cli.out, "Name: %s\n", info.Name)
	ioutils.FprintfIfNotEmpty(cli.out, "ID: %s\n", info.ID)
	fmt.Fprintf(cli.out, "Docker Root Dir: %s\n", info.DockerRootDir)
	fmt.Fprintf(cli.out, "Debug Mode (client): %v\n", utils.IsDebugEnabled())
	fmt.Fprintf(cli.out, "Debug Mode (server): %v\n", info.Debug)

	if info.Debug {
		fmt.Fprintf(cli.out, " File Descriptors: %d\n", info.NFd)
		fmt.Fprintf(cli.out, " Goroutines: %d\n", info.NGoroutines)
		fmt.Fprintf(cli.out, " System Time: %s\n", info.SystemTime)
		fmt.Fprintf(cli.out, " EventsListeners: %d\n", info.NEventsListener)
	}

	ioutils.FprintfIfNotEmpty(cli.out, "Http Proxy: %s\n", info.HTTPProxy)
	ioutils.FprintfIfNotEmpty(cli.out, "Https Proxy: %s\n", info.HTTPSProxy)
	ioutils.FprintfIfNotEmpty(cli.out, "No Proxy: %s\n", info.NoProxy)

	if info.IndexServerAddress != "" {
		u := cli.configFile.AuthConfigs[info.IndexServerAddress].Username
		if len(u) > 0 {
			fmt.Fprintf(cli.out, "Username: %v\n", u)
		}
		fmt.Fprintf(cli.out, "Registry: %v\n", info.IndexServerAddress)
	}

	// Only output these warnings if the server does not support these features
	if info.OSType != "windows" {
		if !info.MemoryLimit {
			fmt.Fprintln(cli.err, "WARNING: No memory limit support")
		}
		if !info.SwapLimit {
			fmt.Fprintln(cli.err, "WARNING: No swap limit support")
		}
		if !info.KernelMemory {
			fmt.Fprintln(cli.err, "WARNING: No kernel memory limit support")
		}
		if !info.OomKillDisable {
			fmt.Fprintln(cli.err, "WARNING: No oom kill disable support")
		}
		if !info.CPUCfsQuota {
			fmt.Fprintln(cli.err, "WARNING: No cpu cfs quota support")
		}
		if !info.CPUCfsPeriod {
			fmt.Fprintln(cli.err, "WARNING: No cpu cfs period support")
		}
		if !info.CPUShares {
			fmt.Fprintln(cli.err, "WARNING: No cpu shares support")
		}
		if !info.CPUSet {
			fmt.Fprintln(cli.err, "WARNING: No cpuset support")
		}
		if !info.IPv4Forwarding {
			fmt.Fprintln(cli.err, "WARNING: IPv4 forwarding is disabled")
		}
		if !info.BridgeNfIptables {
			fmt.Fprintln(cli.err, "WARNING: bridge-nf-call-iptables is disabled")
		}
		if !info.BridgeNfIP6tables {
			fmt.Fprintln(cli.err, "WARNING: bridge-nf-call-ip6tables is disabled")
		}
	}

	if info.Labels != nil {
		fmt.Fprintln(cli.out, "Labels:")
		for _, attribute := range info.Labels {
			fmt.Fprintf(cli.out, " %s\n", attribute)
		}
	}

	ioutils.FprintfIfTrue(cli.out, "Experimental: %v\n", info.ExperimentalBuild)
	if info.ClusterStore != "" {
		fmt.Fprintf(cli.out, "Cluster Store: %s\n", info.ClusterStore)
	}

	if info.ClusterAdvertise != "" {
		fmt.Fprintf(cli.out, "Cluster Advertise: %s\n", info.ClusterAdvertise)
	}

	if info.RegistryConfig != nil && (len(info.RegistryConfig.InsecureRegistryCIDRs) > 0 || len(info.RegistryConfig.IndexConfigs) > 0) {
		fmt.Fprintln(cli.out, "Insecure Registries:")
		for _, registry := range info.RegistryConfig.IndexConfigs {
			if registry.Secure == false {
				fmt.Fprintf(cli.out, " %s\n", registry.Name)
			}
		}

		for _, registry := range info.RegistryConfig.InsecureRegistryCIDRs {
			mask, _ := registry.Mask.Size()
			fmt.Fprintf(cli.out, " %s/%d\n", registry.IP.String(), mask)
		}
	}
	return nil
}
