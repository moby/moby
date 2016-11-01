package container

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

type updateOptions struct {
	blkioWeight        uint16
	cpuPeriod          int64
	cpuQuota           int64
	cpuRealtimePeriod  int64
	cpuRealtimeRuntime int64
	cpusetCpus         string
	cpusetMems         string
	cpuShares          int64
	memoryString       string
	memoryReservation  string
	memorySwap         string
	kernelMemory       string
	restartPolicy      string

	nFlag int

	containers []string
}

// NewUpdateCommand creates a new cobra.Command for `docker update`
func NewUpdateCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts updateOptions

	cmd := &cobra.Command{
		Use:   "update [OPTIONS] CONTAINER [CONTAINER...]",
		Short: "Update configuration of one or more containers",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			opts.nFlag = cmd.Flags().NFlag()
			return runUpdate(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.Uint16Var(&opts.blkioWeight, "blkio-weight", 0, "Block IO (relative weight), between 10 and 1000, or 0 to disable (default 0)")
	flags.Int64Var(&opts.cpuPeriod, "cpu-period", 0, "Limit CPU CFS (Completely Fair Scheduler) period")
	flags.Int64Var(&opts.cpuQuota, "cpu-quota", 0, "Limit CPU CFS (Completely Fair Scheduler) quota")
	flags.Int64Var(&opts.cpuRealtimePeriod, "cpu-rt-period", 0, "Limit the CPU real-time period in microseconds")
	flags.Int64Var(&opts.cpuRealtimeRuntime, "cpu-rt-runtime", 0, "Limit the CPU real-time runtime in microseconds")
	flags.StringVar(&opts.cpusetCpus, "cpuset-cpus", "", "CPUs in which to allow execution (0-3, 0,1)")
	flags.StringVar(&opts.cpusetMems, "cpuset-mems", "", "MEMs in which to allow execution (0-3, 0,1)")
	flags.Int64VarP(&opts.cpuShares, "cpu-shares", "c", 0, "CPU shares (relative weight)")
	flags.StringVarP(&opts.memoryString, "memory", "m", "", "Memory limit")
	flags.StringVar(&opts.memoryReservation, "memory-reservation", "", "Memory soft limit")
	flags.StringVar(&opts.memorySwap, "memory-swap", "", "Swap limit equal to memory plus swap: '-1' to enable unlimited swap")
	flags.StringVar(&opts.kernelMemory, "kernel-memory", "", "Kernel memory limit")
	flags.StringVar(&opts.restartPolicy, "restart", "", "Restart policy to apply when a container exits")

	return cmd
}

func runUpdate(dockerCli *command.DockerCli, opts *updateOptions) error {
	var err error

	if opts.nFlag == 0 {
		return fmt.Errorf("You must provide one or more flags when using this command.")
	}

	var memory int64
	if opts.memoryString != "" {
		memory, err = units.RAMInBytes(opts.memoryString)
		if err != nil {
			return err
		}
	}

	var memoryReservation int64
	if opts.memoryReservation != "" {
		memoryReservation, err = units.RAMInBytes(opts.memoryReservation)
		if err != nil {
			return err
		}
	}

	var memorySwap int64
	if opts.memorySwap != "" {
		if opts.memorySwap == "-1" {
			memorySwap = -1
		} else {
			memorySwap, err = units.RAMInBytes(opts.memorySwap)
			if err != nil {
				return err
			}
		}
	}

	var kernelMemory int64
	if opts.kernelMemory != "" {
		kernelMemory, err = units.RAMInBytes(opts.kernelMemory)
		if err != nil {
			return err
		}
	}

	var restartPolicy containertypes.RestartPolicy
	if opts.restartPolicy != "" {
		restartPolicy, err = runconfigopts.ParseRestartPolicy(opts.restartPolicy)
		if err != nil {
			return err
		}
	}

	resources := containertypes.Resources{
		BlkioWeight:        opts.blkioWeight,
		CpusetCpus:         opts.cpusetCpus,
		CpusetMems:         opts.cpusetMems,
		CPUShares:          opts.cpuShares,
		Memory:             memory,
		MemoryReservation:  memoryReservation,
		MemorySwap:         memorySwap,
		KernelMemory:       kernelMemory,
		CPUPeriod:          opts.cpuPeriod,
		CPUQuota:           opts.cpuQuota,
		CPURealtimePeriod:  opts.cpuRealtimePeriod,
		CPURealtimeRuntime: opts.cpuRealtimeRuntime,
	}

	updateConfig := containertypes.UpdateConfig{
		Resources:     resources,
		RestartPolicy: restartPolicy,
	}

	ctx := context.Background()

	var (
		warns []string
		errs  []string
	)
	for _, container := range opts.containers {
		r, err := dockerCli.Client().ContainerUpdate(ctx, container, updateConfig)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintf(dockerCli.Out(), "%s\n", container)
		}
		warns = append(warns, r.Warnings...)
	}
	if len(warns) > 0 {
		fmt.Fprintf(dockerCli.Out(), "%s", strings.Join(warns, "\n"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
