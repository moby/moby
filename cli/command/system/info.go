package system

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/utils"
	"github.com/docker/docker/utils/templates"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

type infoOptions struct {
	format string
}

// NewInfoCommand creates a new cobra.Command for `docker info`
func NewInfoCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts infoOptions

	cmd := &cobra.Command{
		Use:   "info [OPTIONS]",
		Short: "Display system-wide information",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInfo(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.format, "format", "f", "", "Format the output using the given go template")

	return cmd
}

func runInfo(dockerCli *command.DockerCli, opts *infoOptions) error {
	ctx := context.Background()
	info, err := dockerCli.Client().Info(ctx)
	if err != nil {
		return err
	}
	if opts.format == "" {
		return prettyPrintInfo(dockerCli, info)
	}
	return formatInfo(dockerCli, info, opts.format)
}

func prettyPrintInfo(dockerCli *command.DockerCli, info types.Info) error {
	fmt.Fprintf(dockerCli.Out(), "Containers: %d\n", info.Containers)
	fmt.Fprintf(dockerCli.Out(), " Running: %d\n", info.ContainersRunning)
	fmt.Fprintf(dockerCli.Out(), " Paused: %d\n", info.ContainersPaused)
	fmt.Fprintf(dockerCli.Out(), " Stopped: %d\n", info.ContainersStopped)
	fmt.Fprintf(dockerCli.Out(), "Images: %d\n", info.Images)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Server Version: %s\n", info.ServerVersion)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Storage Driver: %s\n", info.Driver)
	if info.DriverStatus != nil {
		for _, pair := range info.DriverStatus {
			fmt.Fprintf(dockerCli.Out(), " %s: %s\n", pair[0], pair[1])

			// print a warning if devicemapper is using a loopback file
			if pair[0] == "Data loop file" {
				fmt.Fprintln(dockerCli.Err(), " WARNING: Usage of loopback devices is strongly discouraged for production use. Use `--storage-opt dm.thinpooldev` to specify a custom block storage device.")
			}
		}

	}
	if info.SystemStatus != nil {
		for _, pair := range info.SystemStatus {
			fmt.Fprintf(dockerCli.Out(), "%s: %s\n", pair[0], pair[1])
		}
	}
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Logging Driver: %s\n", info.LoggingDriver)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Cgroup Driver: %s\n", info.CgroupDriver)

	fmt.Fprintf(dockerCli.Out(), "Plugins: \n")
	fmt.Fprintf(dockerCli.Out(), " Volume:")
	fmt.Fprintf(dockerCli.Out(), " %s", strings.Join(info.Plugins.Volume, " "))
	fmt.Fprintf(dockerCli.Out(), "\n")
	fmt.Fprintf(dockerCli.Out(), " Network:")
	fmt.Fprintf(dockerCli.Out(), " %s", strings.Join(info.Plugins.Network, " "))
	fmt.Fprintf(dockerCli.Out(), "\n")

	if len(info.Plugins.Authorization) != 0 {
		fmt.Fprintf(dockerCli.Out(), " Authorization:")
		fmt.Fprintf(dockerCli.Out(), " %s", strings.Join(info.Plugins.Authorization, " "))
		fmt.Fprintf(dockerCli.Out(), "\n")
	}

	fmt.Fprintf(dockerCli.Out(), "Swarm: %v\n", info.Swarm.LocalNodeState)
	if info.Swarm.LocalNodeState != swarm.LocalNodeStateInactive {
		fmt.Fprintf(dockerCli.Out(), " NodeID: %s\n", info.Swarm.NodeID)
		if info.Swarm.Error != "" {
			fmt.Fprintf(dockerCli.Out(), " Error: %v\n", info.Swarm.Error)
		}
		fmt.Fprintf(dockerCli.Out(), " Is Manager: %v\n", info.Swarm.ControlAvailable)
		if info.Swarm.ControlAvailable {
			fmt.Fprintf(dockerCli.Out(), " ClusterID: %s\n", info.Swarm.Cluster.ID)
			fmt.Fprintf(dockerCli.Out(), " Managers: %d\n", info.Swarm.Managers)
			fmt.Fprintf(dockerCli.Out(), " Nodes: %d\n", info.Swarm.Nodes)
			fmt.Fprintf(dockerCli.Out(), " Orchestration:\n")
			taskHistoryRetentionLimit := int64(0)
			if info.Swarm.Cluster.Spec.Orchestration.TaskHistoryRetentionLimit != nil {
				taskHistoryRetentionLimit = *info.Swarm.Cluster.Spec.Orchestration.TaskHistoryRetentionLimit
			}
			fmt.Fprintf(dockerCli.Out(), "  Task History Retention Limit: %d\n", taskHistoryRetentionLimit)
			fmt.Fprintf(dockerCli.Out(), " Raft:\n")
			fmt.Fprintf(dockerCli.Out(), "  Snapshot Interval: %d\n", info.Swarm.Cluster.Spec.Raft.SnapshotInterval)
			fmt.Fprintf(dockerCli.Out(), "  Heartbeat Tick: %d\n", info.Swarm.Cluster.Spec.Raft.HeartbeatTick)
			fmt.Fprintf(dockerCli.Out(), "  Election Tick: %d\n", info.Swarm.Cluster.Spec.Raft.ElectionTick)
			fmt.Fprintf(dockerCli.Out(), " Dispatcher:\n")
			fmt.Fprintf(dockerCli.Out(), "  Heartbeat Period: %s\n", units.HumanDuration(time.Duration(info.Swarm.Cluster.Spec.Dispatcher.HeartbeatPeriod)))
			fmt.Fprintf(dockerCli.Out(), " CA Configuration:\n")
			fmt.Fprintf(dockerCli.Out(), "  Expiry Duration: %s\n", units.HumanDuration(info.Swarm.Cluster.Spec.CAConfig.NodeCertExpiry))
			if len(info.Swarm.Cluster.Spec.CAConfig.ExternalCAs) > 0 {
				fmt.Fprintf(dockerCli.Out(), "  External CAs:\n")
				for _, entry := range info.Swarm.Cluster.Spec.CAConfig.ExternalCAs {
					fmt.Fprintf(dockerCli.Out(), "    %s: %s\n", entry.Protocol, entry.URL)
				}
			}
		}
		fmt.Fprintf(dockerCli.Out(), " Node Address: %s\n", info.Swarm.NodeAddr)
	}

	if len(info.Runtimes) > 0 {
		fmt.Fprintf(dockerCli.Out(), "Runtimes:")
		for name := range info.Runtimes {
			fmt.Fprintf(dockerCli.Out(), " %s", name)
		}
		fmt.Fprint(dockerCli.Out(), "\n")
		fmt.Fprintf(dockerCli.Out(), "Default Runtime: %s\n", info.DefaultRuntime)
	}

	if info.OSType == "linux" {
		fmt.Fprintf(dockerCli.Out(), "Security Options:")
		ioutils.FprintfIfNotEmpty(dockerCli.Out(), " %s", strings.Join(info.SecurityOptions, " "))
		fmt.Fprintf(dockerCli.Out(), "\n")
	}

	// Isolation only has meaning on a Windows daemon.
	if info.OSType == "windows" {
		fmt.Fprintf(dockerCli.Out(), "Default Isolation: %v\n", info.Isolation)
	}

	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Kernel Version: %s\n", info.KernelVersion)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Operating System: %s\n", info.OperatingSystem)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "OSType: %s\n", info.OSType)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Architecture: %s\n", info.Architecture)
	fmt.Fprintf(dockerCli.Out(), "CPUs: %d\n", info.NCPU)
	fmt.Fprintf(dockerCli.Out(), "Total Memory: %s\n", units.BytesSize(float64(info.MemTotal)))
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Name: %s\n", info.Name)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "ID: %s\n", info.ID)
	fmt.Fprintf(dockerCli.Out(), "Docker Root Dir: %s\n", info.DockerRootDir)
	fmt.Fprintf(dockerCli.Out(), "Debug Mode (client): %v\n", utils.IsDebugEnabled())
	fmt.Fprintf(dockerCli.Out(), "Debug Mode (server): %v\n", info.Debug)

	if info.Debug {
		fmt.Fprintf(dockerCli.Out(), " File Descriptors: %d\n", info.NFd)
		fmt.Fprintf(dockerCli.Out(), " Goroutines: %d\n", info.NGoroutines)
		fmt.Fprintf(dockerCli.Out(), " System Time: %s\n", info.SystemTime)
		fmt.Fprintf(dockerCli.Out(), " EventsListeners: %d\n", info.NEventsListener)
	}

	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Http Proxy: %s\n", info.HTTPProxy)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Https Proxy: %s\n", info.HTTPSProxy)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "No Proxy: %s\n", info.NoProxy)

	if info.IndexServerAddress != "" {
		u := dockerCli.ConfigFile().AuthConfigs[info.IndexServerAddress].Username
		if len(u) > 0 {
			fmt.Fprintf(dockerCli.Out(), "Username: %v\n", u)
		}
		fmt.Fprintf(dockerCli.Out(), "Registry: %v\n", info.IndexServerAddress)
	}

	// Only output these warnings if the server does not support these features
	if info.OSType != "windows" {
		if !info.MemoryLimit {
			fmt.Fprintln(dockerCli.Err(), "WARNING: No memory limit support")
		}
		if !info.SwapLimit {
			fmt.Fprintln(dockerCli.Err(), "WARNING: No swap limit support")
		}
		if !info.KernelMemory {
			fmt.Fprintln(dockerCli.Err(), "WARNING: No kernel memory limit support")
		}
		if !info.OomKillDisable {
			fmt.Fprintln(dockerCli.Err(), "WARNING: No oom kill disable support")
		}
		if !info.CPUCfsQuota {
			fmt.Fprintln(dockerCli.Err(), "WARNING: No cpu cfs quota support")
		}
		if !info.CPUCfsPeriod {
			fmt.Fprintln(dockerCli.Err(), "WARNING: No cpu cfs period support")
		}
		if !info.CPUShares {
			fmt.Fprintln(dockerCli.Err(), "WARNING: No cpu shares support")
		}
		if !info.CPUSet {
			fmt.Fprintln(dockerCli.Err(), "WARNING: No cpuset support")
		}
		if !info.IPv4Forwarding {
			fmt.Fprintln(dockerCli.Err(), "WARNING: IPv4 forwarding is disabled")
		}
		if !info.BridgeNfIptables {
			fmt.Fprintln(dockerCli.Err(), "WARNING: bridge-nf-call-iptables is disabled")
		}
		if !info.BridgeNfIP6tables {
			fmt.Fprintln(dockerCli.Err(), "WARNING: bridge-nf-call-ip6tables is disabled")
		}
	}

	if info.Labels != nil {
		fmt.Fprintln(dockerCli.Out(), "Labels:")
		for _, attribute := range info.Labels {
			fmt.Fprintf(dockerCli.Out(), " %s\n", attribute)
		}
	}

	ioutils.FprintfIfTrue(dockerCli.Out(), "Experimental: %v\n", info.ExperimentalBuild)
	if info.ClusterStore != "" {
		fmt.Fprintf(dockerCli.Out(), "Cluster Store: %s\n", info.ClusterStore)
	}

	if info.ClusterAdvertise != "" {
		fmt.Fprintf(dockerCli.Out(), "Cluster Advertise: %s\n", info.ClusterAdvertise)
	}

	if info.RegistryConfig != nil && (len(info.RegistryConfig.InsecureRegistryCIDRs) > 0 || len(info.RegistryConfig.IndexConfigs) > 0) {
		fmt.Fprintln(dockerCli.Out(), "Insecure Registries:")
		for _, registry := range info.RegistryConfig.IndexConfigs {
			if registry.Secure == false {
				fmt.Fprintf(dockerCli.Out(), " %s\n", registry.Name)
			}
		}

		for _, registry := range info.RegistryConfig.InsecureRegistryCIDRs {
			mask, _ := registry.Mask.Size()
			fmt.Fprintf(dockerCli.Out(), " %s/%d\n", registry.IP.String(), mask)
		}
	}

	if info.RegistryConfig != nil && len(info.RegistryConfig.Mirrors) > 0 {
		fmt.Fprintln(dockerCli.Out(), "Registry Mirrors:")
		for _, mirror := range info.RegistryConfig.Mirrors {
			fmt.Fprintf(dockerCli.Out(), " %s\n", mirror)
		}
	}

	fmt.Fprintf(dockerCli.Out(), "Live Restore Enabled: %v\n", info.LiveRestoreEnabled)

	return nil
}

func formatInfo(dockerCli *command.DockerCli, info types.Info, format string) error {
	tmpl, err := templates.Parse(format)
	if err != nil {
		return cli.StatusError{StatusCode: 64,
			Status: "Template parsing error: " + err.Error()}
	}
	err = tmpl.Execute(dockerCli.Out(), info)
	dockerCli.Out().Write([]byte{'\n'})
	return err
}
