package metricdriver

import (
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/cgroups/fs"
)

var (
	metricSubsystems = []string{"memory", "cpuacct"}
)

func Get(id, parent string, pid int) (map[string]map[string]float64, error) {

	metric := make(map[string]map[string]float64)

	c := &cgroups.Cgroup{
		Name:   id,
		Parent: parent,
	}

	for _, subsystem := range metricSubsystems {
		stat, err := fs.GetStats(c, subsystem, pid)
		if err != nil {
			return nil, err
		}
		metric[subsystem] = stat
	}

	return metric, nil
}
