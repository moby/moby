package client

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/go-units"
)

// CmdUpdate updates resources of one or more containers.
//
// Usage: docker update [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdUpdate(args ...string) error {
	cmd := Cli.Subcmd("update", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["update"].Description, true)
	flBlkioWeight := cmd.Uint16([]string{"-blkio-weight"}, 0, "Block IO (relative weight), between 10 and 1000")
	flCPUPeriod := cmd.Int64([]string{"-cpu-period"}, 0, "Limit CPU CFS (Completely Fair Scheduler) period")
	flCPUQuota := cmd.Int64([]string{"-cpu-quota"}, 0, "Limit CPU CFS (Completely Fair Scheduler) quota")
	flCpusetCpus := cmd.String([]string{"-cpuset-cpus"}, "", "CPUs in which to allow execution (0-3, 0,1)")
	flCpusetMems := cmd.String([]string{"-cpuset-mems"}, "", "MEMs in which to allow execution (0-3, 0,1)")
	flCPUShares := cmd.Int64([]string{"c", "-cpu-shares"}, 0, "CPU shares (relative weight)")
	flMemoryString := cmd.String([]string{"m", "-memory"}, "", "Memory limit")
	flMemoryReservation := cmd.String([]string{"-memory-reservation"}, "", "Memory soft limit")
	flMemorySwap := cmd.String([]string{"-memory-swap"}, "", "Swap limit equal to memory plus swap: '-1' to enable unlimited swap")
	flKernelMemory := cmd.String([]string{"-kernel-memory"}, "", "Kernel memory limit")
	flRestartPolicy := cmd.String([]string{"-restart"}, "", "Restart policy to apply when a container exits")

	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)
	if cmd.NFlag() == 0 {
		return fmt.Errorf("You must provide one or more flags when using this command.")
	}

	var err error
	var flMemory int64
	if *flMemoryString != "" {
		flMemory, err = units.RAMInBytes(*flMemoryString)
		if err != nil {
			return err
		}
	}

	var memoryReservation int64
	if *flMemoryReservation != "" {
		memoryReservation, err = units.RAMInBytes(*flMemoryReservation)
		if err != nil {
			return err
		}
	}

	var memorySwap int64
	if *flMemorySwap != "" {
		if *flMemorySwap == "-1" {
			memorySwap = -1
		} else {
			memorySwap, err = units.RAMInBytes(*flMemorySwap)
			if err != nil {
				return err
			}
		}
	}

	var kernelMemory int64
	if *flKernelMemory != "" {
		kernelMemory, err = units.RAMInBytes(*flKernelMemory)
		if err != nil {
			return err
		}
	}

	var restartPolicy container.RestartPolicy
	if *flRestartPolicy != "" {
		restartPolicy, err = opts.ParseRestartPolicy(*flRestartPolicy)
		if err != nil {
			return err
		}
	}

	resources := container.Resources{
		BlkioWeight:       *flBlkioWeight,
		CpusetCpus:        *flCpusetCpus,
		CpusetMems:        *flCpusetMems,
		CPUShares:         *flCPUShares,
		Memory:            flMemory,
		MemoryReservation: memoryReservation,
		MemorySwap:        memorySwap,
		KernelMemory:      kernelMemory,
		CPUPeriod:         *flCPUPeriod,
		CPUQuota:          *flCPUQuota,
	}

	updateConfig := container.UpdateConfig{
		Resources:     resources,
		RestartPolicy: restartPolicy,
	}

	ctx := context.Background()

	names := cmd.Args()
	var errs []string

	for _, name := range names {
		if err := cli.client.ContainerUpdate(ctx, name, updateConfig); err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}

	return nil
}
