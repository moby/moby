package client

import (
	"fmt"
	"net/url"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/runconfig"
)

type nestedHostConfig struct {
	origHostConfig runconfig.HostConfig `json:"HostConfig"`
	origConfig     runconfig.Config     `json:"Config"`
}

// CmdModResources is used to modify resource allocations to a running(?) container.
//
// Usage: docker modresources --container CONTAINER OPTION [OPTIONS]
// This (storing the name of the container in config.image) is a weird hack. Fix this
func (cli *DockerCli) CmdModresources(args ...string) error {
	cmd := Cli.Subcmd("modresources", []string{"--container CONTAINER OPTION [OPTIONS]"}, "Modify resource allocations to a running(?) container", true)
	addTrustedFlags(cmd, true)

	fmt.Println("Here")

	var (
		flid             = cmd.String([]string{"-container"}, "", "Container ID")
		flCPUShares      = cmd.Int64([]string{"#c", "-cpu-shares"}, -1, "CPU shares (relative weight)")
		flCPUPeriod      = cmd.Int64([]string{"-cpu-period"}, -1, "Limit CPU CFS (Completely Fair Scheduler) period")
		flCPUQuota       = cmd.Int64([]string{"-cpu-quota"}, -1, "Limit CPU CFS (Completely Fair Scheduler) quota")
		flCpusetCpus     = cmd.String([]string{"#-cpuset", "-cpuset-cpus"}, "notset", "CPUs in which to allow execution (0-3, 0,1)")
		flCpusetMems     = cmd.String([]string{"-cpuset-mems"}, "notset", "MEMs in which to allow execution (0-3, 0,1)")
		flBlkioWeight    = cmd.Int64([]string{"-blkio-weight"}, -1, "Block IO (relative weight), between 10 and 1000")
		flBlkioReadLimit = cmd.String([]string{"-blkio-read-limit"}, "notset", "Block IO read limit, in bytes per second. Default unlimited")
		flSwappiness     = cmd.Int64([]string{"-memory-swappiness"}, -1, "Tuning container memory swappiness (0 to 100)")
	)

	cmd.ParseFlags(args, true)

	fmt.Println("Container ID given: ", *flid)

	if *flid == "" {
		cmd.Usage()
		return nil
	}

	var newHostConfig runconfig.HostConfig
	var newConfig runconfig.Config

	containerValues := url.Values{}
	containerValues.Set("ID", *flid)

	newHostConfig.CPUShares = *flCPUShares
	newHostConfig.CPUPeriod = *flCPUPeriod
	newHostConfig.CpusetCpus = *flCpusetCpus
	newHostConfig.CpusetMems = *flCpusetMems
	newHostConfig.CPUQuota = *flCPUQuota
	newHostConfig.BlkioWeight = *flBlkioWeight
	newHostConfig.BlkioReadLimit = *flBlkioReadLimit
	newHostConfig.MemorySwappiness = flSwappiness

	fmt.Printf("newHostConfig%+v\n", newHostConfig)

	if _, err := cli.call("POST", "/containers/modresources?"+containerValues.Encode(), runconfig.MergeConfigs(&newConfig, &newHostConfig), nil); err != nil {
		return nil
	}

	return nil
}
