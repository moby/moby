package client

import (
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/httputils"
	"github.com/docker/docker/pkg/ioutils"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/units"
)

// CmdInfo displays system-wide information.
//
// Usage: docker info
func (cli *DockerCli) CmdInfo(args ...string) error {
	cmd := Cli.Subcmd("info", nil, Cli.DockerCommands["info"].Description, true)
	cmd.Require(flag.Exact, 0)

	cmd.ParseFlags(args, true)

	serverResp, err := cli.call("GET", "/info", nil, nil)
	if err != nil {
		return err
	}

	defer serverResp.body.Close()

	info := &types.Info{}
	if err := json.NewDecoder(serverResp.body).Decode(info); err != nil {
		return fmt.Errorf("Error reading remote info: %v", err)
	}

	fmt.Fprintf(cli.out, "Containers: %d\n", info.Containers)
	fmt.Fprintf(cli.out, "Images: %d\n", info.Images)
	ioutils.FprintfIfNotEmpty(cli.out, "Server Version: %s\n", info.ServerVersion)
	ioutils.FprintfIfNotEmpty(cli.out, "Storage Driver: %s\n", info.Driver)
	if info.DriverStatus != nil {
		for _, pair := range info.DriverStatus {
			fmt.Fprintf(cli.out, " %s: %s\n", pair[0], pair[1])
		}
	}
	ioutils.FprintfIfNotEmpty(cli.out, "Execution Driver: %s\n", info.ExecutionDriver)
	ioutils.FprintfIfNotEmpty(cli.out, "Logging Driver: %s\n", info.LoggingDriver)

	fmt.Fprintf(cli.out, "Plugins: \n")
	fmt.Fprintf(cli.out, " Volume:")
	for _, driver := range info.Plugins.Volume {
		fmt.Fprintf(cli.out, " %s", driver)
	}
	fmt.Fprintf(cli.out, "\n")
	fmt.Fprintf(cli.out, " Network:")
	for _, driver := range info.Plugins.Network {
		fmt.Fprintf(cli.out, " %s", driver)
	}
	fmt.Fprintf(cli.out, "\n")

	ioutils.FprintfIfNotEmpty(cli.out, "Kernel Version: %s\n", info.KernelVersion)
	ioutils.FprintfIfNotEmpty(cli.out, "Operating System: %s\n", info.OperatingSystem)
	ioutils.FprintfIfNotEmpty(cli.out, "OSType: %s\n", info.OSType)
	ioutils.FprintfIfNotEmpty(cli.out, "Architecture: %s\n", info.Architecture)
	fmt.Fprintf(cli.out, "CPUs: %d\n", info.NCPU)
	fmt.Fprintf(cli.out, "Total Memory: %s\n", units.BytesSize(float64(info.MemTotal)))
	ioutils.FprintfIfNotEmpty(cli.out, "Name: %s\n", info.Name)
	ioutils.FprintfIfNotEmpty(cli.out, "ID: %s\n", info.ID)

	if info.Debug {
		fmt.Fprintf(cli.out, "Debug mode (server): %v\n", info.Debug)
		fmt.Fprintf(cli.out, " File Descriptors: %d\n", info.NFd)
		fmt.Fprintf(cli.out, " Goroutines: %d\n", info.NGoroutines)
		fmt.Fprintf(cli.out, " System Time: %s\n", info.SystemTime)
		fmt.Fprintf(cli.out, " EventsListeners: %d\n", info.NEventsListener)
		fmt.Fprintf(cli.out, " Init SHA1: %s\n", info.InitSha1)
		fmt.Fprintf(cli.out, " Init Path: %s\n", info.InitPath)
		fmt.Fprintf(cli.out, " Docker Root Dir: %s\n", info.DockerRootDir)
	}

	ioutils.FprintfIfNotEmpty(cli.out, "Http Proxy: %s\n", info.HTTPProxy)
	ioutils.FprintfIfNotEmpty(cli.out, "Https Proxy: %s\n", info.HTTPSProxy)
	ioutils.FprintfIfNotEmpty(cli.out, "No Proxy: %s\n", info.NoProxy)

	if info.IndexServerAddress != "" {
		u := cli.configFile.AuthConfigs[info.IndexServerAddress].Username
		if len(u) > 0 {
			fmt.Fprintf(cli.out, "Username: %v\n", u)
			fmt.Fprintf(cli.out, "Registry: %v\n", info.IndexServerAddress)
		}
	}

	// Only output these warnings if the server does not support these features
	if h, err := httputils.ParseServerHeader(serverResp.header.Get("Server")); err == nil {
		if h.OS != "windows" {
			if !info.MemoryLimit {
				fmt.Fprintf(cli.err, "WARNING: No memory limit support\n")
			}
			if !info.SwapLimit {
				fmt.Fprintf(cli.err, "WARNING: No swap limit support\n")
			}
			if !info.OomKillDisable {
				fmt.Fprintf(cli.err, "WARNING: No oom kill disable support\n")
			}
			if !info.CPUCfsQuota {
				fmt.Fprintf(cli.err, "WARNING: No cpu cfs quota support\n")
			}
			if !info.CPUCfsPeriod {
				fmt.Fprintf(cli.err, "WARNING: No cpu cfs period support\n")
			}
			if !info.CPUShares {
				fmt.Fprintf(cli.err, "WARNING: No cpu shares support\n")
			}
			if !info.CPUSet {
				fmt.Fprintf(cli.err, "WARNING: No cpuset support\n")
			}
			if !info.IPv4Forwarding {
				fmt.Fprintf(cli.err, "WARNING: IPv4 forwarding is disabled\n")
			}
			if !info.BridgeNfIptables {
				fmt.Fprintf(cli.err, "WARNING: bridge-nf-call-iptables is disabled\n")
			}
			if !info.BridgeNfIP6tables {
				fmt.Fprintf(cli.err, "WARNING: bridge-nf-call-ip6tables is disabled\n")
			}
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
		fmt.Fprintf(cli.out, "Cluster store: %s\n", info.ClusterStore)
	}

	if info.ClusterAdvertise != "" {
		fmt.Fprintf(cli.out, "Cluster advertise: %s\n", info.ClusterAdvertise)
	}
	return nil
}
