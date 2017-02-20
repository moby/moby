package daemon

import (
	"fmt"
	"strconv"
	"strings"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/go-units"
)

func (daemon *Daemon) normalizeStandaloneAndClusterSettings(container *container.Container, config *containertypes.Config, hostConfig *containertypes.HostConfig) error {
	if v, ok := config.Labels["com.docker.resource.climit.pids"]; ok {
		val, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot recognize pids value: %s", v)
		}
		hostConfig.Resources.PidsLimit = val
	}

	if v, ok := config.Labels["com.docker.resource.climit.cpu_shares"]; ok {
		val, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot recognize cpu_shares value: %s", v)
		}
		hostConfig.Resources.CPUShares = val
	}

	if v, ok := config.Labels["com.docker.resource.climit.blkio_weight"]; ok {
		val, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return fmt.Errorf("cannot recognize blkio_weight value: %s", v)
		}
		hostConfig.Resources.BlkioWeight = uint16(val)
	}

	if v, ok := config.Labels["com.docker.resource.climit.memsw"]; ok {
		val, err := units.RAMInBytes(v)
		if err != nil {
			return fmt.Errorf("cannot recognize memsw value: %s", v)
		}
		hostConfig.Resources.MemorySwap = val
	}

	if v, ok := config.Labels["com.docker.resource.climit.oom_kill_disable"]; ok {
		val, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("cannot recognize oom_kill_disable value: %s", v)
		}
		hostConfig.Resources.OomKillDisable = &val
	}

	if v, ok := config.Labels["com.docker.resource.ulimit.opts"]; ok {
		opts := strings.Split(v, ",")
		if hostConfig.Resources.Ulimits == nil {
			hostConfig.Resources.Ulimits = []*units.Ulimit{}
		}
		for id := range opts {
			opt, err := units.ParseUlimit(opts[id])
			if err != nil {
				return err
			}
			hostConfig.Resources.Ulimits = append(hostConfig.Resources.Ulimits, opt)
		}
	}

	if v, ok := config.Labels["com.docker.storage.opts"]; ok {
		opts := strings.Split(v, ",")
		if hostConfig.StorageOpt == nil {
			hostConfig.StorageOpt = make(map[string]string)
		}
		for id := range opts {
			pair := strings.SplitN(opts[id], "=", 2)
			if len(pair) != 2 {
				return fmt.Errorf("cannot recognize storage option value: %s", pair)
			}
			hostConfig.StorageOpt[pair[0]] = pair[1]
		}
	}

	if v, ok := config.Labels["com.docker.volume.default.disable_anonymous"]; ok {
		val, err := strconv.ParseBool(v)
		if err == nil && val {
			config.Volumes = make(map[string]struct{})
		}
	}

	if v, ok := config.Labels["com.docker.volume.default.driver"]; ok {
		hostConfig.VolumeDriver = v
	}

	return nil
}
