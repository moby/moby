package client

import (
	// "encoding/base64"
	// "encoding/json"
	"fmt"
	// "io"
	// "bytes"
	"net/url"
	"strconv"
	// "os"
	// "strings"
	// "reflect"

	// "github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	// "github.com/docker/docker/graph/tags"
	// "github.com/docker/docker/pkg/parsers"
	// "github.com/docker/docker/registry"
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
	)

	cmd.ParseFlags(args, true)

	fmt.Println("Container ID given: ", *flid)

	if *flid == "" {
		cmd.Usage()
		return nil
	}

	// obj, _, _ := readBody(cli.call("GET", "/containers/ "+(*flid)+"/json", nil, nil))
	// var data nestedHostConfig
	// err := json.Unmarshal(obj, &data)
	// if err != nil {
	// 	fmt.Println("error:", err)
	// }

	// rdr := bytes.NewReader(obj)
	// dec := json.NewDecoder(rdr)

	// dec.Decode(&data)

	// fmt.Printf("%+v\n\n", data)

	// hostConfig := &runconfig.HostConfig{
	// 	Binds:            data.origHostConfig.Binds,
	// 	ContainerIDFile:  data.origHostConfig.ContainerIDFile,
	// 	LxcConf:          data.origHostConfig.LxcConf,
	// 	Memory:           data.origHostConfig.Memory,
	// 	MemorySwap:       data.origHostConfig.MemorySwap,
	// 	KernelMemory:     data.origHostConfig.KernelMemory,
	// 	CPUShares:        *flCPUShares,
	// 	CPUPeriod:        *flCPUPeriod,
	// 	CpusetCpus:       *flCpusetCpus,
	// 	CpusetMems:       *flCpusetMems,
	// 	CPUQuota:         *flCPUQuota,
	// 	BlkioWeight:      *flBlkioWeight,
	// 	BlkioReadLimit:   *flBlkioReadLimit,
	// 	OomKillDisable:   data.origHostConfig.OomKillDisable,
	// 	MemorySwappiness: data.origHostConfig.MemorySwappiness,
	// 	Privileged:       data.origHostConfig.Privileged,
	// 	PortBindings:     data.origHostConfig.PortBindings,
	// 	Links:            data.origHostConfig.Links,
	// 	PublishAllPorts:  data.origHostConfig.PublishAllPorts,
	// 	DNS:              data.origHostConfig.DNS,
	// 	DNSSearch:        data.origHostConfig.DNSSearch,
	// 	DNSOptions:       data.origHostConfig.DNSOptions,
	// 	ExtraHosts:       data.origHostConfig.ExtraHosts,
	// 	VolumesFrom:      data.origHostConfig.VolumesFrom,
	// 	NetworkMode:      data.origHostConfig.NetworkMode,
	// 	IpcMode:          data.origHostConfig.IpcMode,
	// 	PidMode:          data.origHostConfig.PidMode,
	// 	UTSMode:          data.origHostConfig.UTSMode,
	// 	Devices:          data.origHostConfig.Devices,
	// 	CapAdd:           data.origHostConfig.CapAdd,
	// 	CapDrop:          data.origHostConfig.CapDrop,
	// 	GroupAdd:         data.origHostConfig.GroupAdd,
	// 	RestartPolicy:    data.origHostConfig.RestartPolicy,
	// 	SecurityOpt:      data.origHostConfig.SecurityOpt,
	// 	ReadonlyRootfs:   data.origHostConfig.ReadonlyRootfs,
	// 	Ulimits:          data.origHostConfig.Ulimits,
	// 	LogConfig:        data.origHostConfig.LogConfig,
	// 	CgroupParent:     data.origHostConfig.CgroupParent,
	// 	VolumeDriver:     data.origHostConfig.VolumeDriver,
	// }

	var newHostConfig runconfig.HostConfig
	var newConfig runconfig.Config

	containerValues := url.Values{}
	containerValues.Set("ID", *flid)

	if (*flCPUShares) != -1 {
		containerValues.Set("CPUShares", strconv.FormatInt(*flCPUShares, 10))
		newHostConfig.CPUShares = *flCPUShares
	}
	if (*flCPUPeriod) != -1 {
		containerValues.Set("CPUPeriod", strconv.FormatInt(*flCPUPeriod, 10))
		newHostConfig.CPUPeriod = *flCPUPeriod
	}
	if (*flCpusetCpus) != "notset" {
		containerValues.Set("CpusetCpus", *flCpusetCpus)
		newHostConfig.CpusetCpus = *flCpusetCpus
		// CpusetCpus = data.origCpusetCpus
	}
	if (*flCpusetMems) != "notset" {
		containerValues.Set("CpusetMems", *flCpusetMems)
		newHostConfig.CpusetMems = *flCpusetMems
		// CpusetMems = data.origCpusetMems
	}
	if (*flCPUQuota) != -1 {
		containerValues.Set("CPUQuota", strconv.FormatInt(*flCPUQuota, 10))
		newHostConfig.CPUQuota = *flCPUQuota
		// CPUQuota = data.origCPUQuota
	}
	if (*flBlkioWeight) != -1 {
		containerValues.Set("BlkioWeight", strconv.FormatInt(*flBlkioWeight, 10))
		newHostConfig.BlkioWeight = *flBlkioWeight
		// BlkioWeight = data.origBlkioWeight
	}
	if (*flBlkioReadLimit) != "notset" {
		containerValues.Set("BlkioReadLimit", *flBlkioReadLimit)
		newHostConfig.BlkioReadLimit = *flBlkioReadLimit
		// BlkioReadLimit = data.origBlkioReadLimit
	}

	fmt.Printf("newHostConfig%+v\n", newHostConfig)

	// mergedConfig := runconfig.MergeConfigs(&data.origConfig, hostConfig)

	fmt.Println("URL: ", "/containers/modresources?"+containerValues.Encode())

	// if _, err := cli.call("POST", "/containers/modresources?"+containerValues.Encode(), nil, nil); err != nil {
	// 	return nil
	// }

	if _, err := cli.call("POST", "/containers/modresources?"+containerValues.Encode(), runconfig.MergeConfigs(&newConfig, &newHostConfig), nil); err != nil {
		return nil
	}

	return nil
}
