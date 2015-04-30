package client

import (
	"fmt"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/units"
	"github.com/docker/docker/runconfig"
)

func (cli *DockerCli) CmdSet(args ...string) error {
	cmd := cli.Subcmd("set", "CONTAINER [CONTAINER...]", "Setup/Update resource configs for a container", true)
	flCpusetCpus := cmd.String([]string{"#-cpuset", "-cpuset-cpus"}, "", "CPUs in which to allow execution (0-3, 0,1)")
	flCpusetMems := cmd.String([]string{"-cpuset-mems"}, "", "MEMs in which to allow execution (0-3, 0,1)")
	flCpuShares := cmd.Int64([]string{"c", "-cpu-shares"}, 0, "CPU shares (relative weight)")
	flMemoryString := cmd.String([]string{"m", "-memory"}, "", "Memory limit")
	flMemorySwap := cmd.String([]string{"-memory-swap"}, "", "Total memory (memory + swap), '-1' to disable swap")
	flCpuQuota := cmd.Int64([]string{"-cpu-quota"}, 0, "Limit the CPU CFS (Completely Fair Scheduler) quota")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var flMemory int64
	if *flMemoryString != "" {
		parsedMemory, err := units.RAMInBytes(*flMemoryString)
		if err != nil {
			return err
		}
		flMemory = parsedMemory
	}

	var MemorySwap int64
	if *flMemorySwap != "" {
		if *flMemorySwap == "-1" {
			MemorySwap = -1
		} else {
			parsedMemorySwap, err := units.RAMInBytes(*flMemorySwap)
			if err != nil {
				return err
			}
			MemorySwap = parsedMemorySwap
		}
	}

	hostConfig := runconfig.HostConfig{
		CpusetCpus: *flCpusetCpus,
		CpusetMems: *flCpusetMems,
		CpuShares:  *flCpuShares,
		Memory:     flMemory,
		MemorySwap: MemorySwap,
		CpuQuota:   *flCpuQuota,
	}

	names := cmd.Args()
	var encounteredError error
	for _, name := range names {
		_, _, err := readBody(cli.call("POST", "/containers/"+name+"/set", hostConfig, nil))
		if err != nil {
			fmt.Fprintf(cli.err, "Error trying to set properties of container (%s): %s\n", name, err)
			encounteredError = fmt.Errorf("Error: failed to set one or more containers")
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}

	return encounteredError
}
