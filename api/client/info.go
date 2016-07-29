package client

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/ioutils"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/go-units"
)

// CmdInfo displays system-wide information.
//
// Usage: docker info
func (cli *DockerCli) CmdInfo(args ...string) error {
	cmd := Cli.Subcmd("info", nil, Cli.DockerCommands["info"].Description, true)
	cmd.Require(flag.Exact, 0)

	cmd.ParseFlags(args, true)

	ctx := context.Background()
	info, err := cli.client.Info(ctx)
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
				fmt.Fprintln(cli.err, " WARNING: Usage of loopback devices is strongly discouraged for production use. Use `--storage-opt dm.thinpooldev` to specify a custom block storage device.")
			}
		}

	}
	if info.SystemStatus != nil {
		for _, pair := range info.SystemStatus {
			fmt.Fprintf(cli.out, "%s: %s\n", pair[0], pair[1])
		}
	}
	ioutils.FprintfIfNotEmpty(cli.out, "Logging Driver: %s\n", info.LoggingDriver)
	ioutils.FprintfIfNotEmpty(cli.out, "Cgroup Driver: %s\n", info.CgroupDriver)

	fmt.Fprintf(cli.out, "Plugins:\n")
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

	fmt.Fprintf(cli.out, "Swarm: %v\n", info.Swarm.LocalNodeState)
	if info.Swarm.LocalNodeState != swarm.LocalNodeStateInactive {
		fmt.Fprintf(cli.out, " NodeID: %s\n", info.Swarm.NodeID)
		if info.Swarm.Error != "" {
			fmt.Fprintf(cli.out, " Error: %v\n", info.Swarm.Error)
		}
		fmt.Fprintf(cli.out, " Is Manager: %v\n", info.Swarm.ControlAvailable)
		if info.Swarm.ControlAvailable {
			fmt.Fprintf(cli.out, " ClusterID: %s\n", info.Swarm.Cluster.ID)
			fmt.Fprintf(cli.out, " Managers: %d\n", info.Swarm.Managers)
			fmt.Fprintf(cli.out, " Nodes: %d\n", info.Swarm.Nodes)
			fmt.Fprintf(cli.out, " Orchestration:\n")
			fmt.Fprintf(cli.out, "  Task History Retention Limit: %d\n", info.Swarm.Cluster.Spec.Orchestration.TaskHistoryRetentionLimit)
			fmt.Fprintf(cli.out, " Raft:\n")
			fmt.Fprintf(cli.out, "  Snapshot Interval: %d\n", info.Swarm.Cluster.Spec.Raft.SnapshotInterval)
			fmt.Fprintf(cli.out, "  Heartbeat Tick: %d\n", info.Swarm.Cluster.Spec.Raft.HeartbeatTick)
			fmt.Fprintf(cli.out, "  Election Tick: %d\n", info.Swarm.Cluster.Spec.Raft.ElectionTick)
			fmt.Fprintf(cli.out, " Dispatcher:\n")
			fmt.Fprintf(cli.out, "  Heartbeat Period: %s\n", units.HumanDuration(time.Duration(info.Swarm.Cluster.Spec.Dispatcher.HeartbeatPeriod)))
			fmt.Fprintf(cli.out, " CA Configuration:\n")
			fmt.Fprintf(cli.out, "  Expiry Duration: %s\n", units.HumanDuration(info.Swarm.Cluster.Spec.CAConfig.NodeCertExpiry))
			if len(info.Swarm.Cluster.Spec.CAConfig.ExternalCAs) > 0 {
				fmt.Fprintf(cli.out, "  External CAs:\n")
				for _, entry := range info.Swarm.Cluster.Spec.CAConfig.ExternalCAs {
					fmt.Fprintf(cli.out, "    %s: %s\n", entry.Protocol, entry.URL)
				}
			}
		}
		fmt.Fprintf(cli.out, " Node Address: %s\n", info.Swarm.NodeAddr)
	}

	if len(info.Runtimes) > 0 {
		fmt.Fprintf(cli.out, "Runtimes:")
		for name := range info.Runtimes {
			fmt.Fprintf(cli.out, " %s", name)
		}
		fmt.Fprint(cli.out, "\n")
		fmt.Fprintf(cli.out, "Default Runtime: %s\n", info.DefaultRuntime)
	}

	fmt.Fprintf(cli.out, "Security Options:")
	ioutils.FprintfIfNotEmpty(cli.out, " %s", strings.Join(info.SecurityOptions, " "))
	fmt.Fprintf(cli.out, "\n")

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
