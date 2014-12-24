package metricdriver

import (
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/cgroups/fs"
)

func Get(id, parent string, pid int) (*cgroups.Stats, error) {

	c := &cgroups.Cgroup{
		Name:   id,
		Parent: parent,
	}

	stats, err := fs.GetAllStats(c, pid)

	if err != nil {
		return nil, err
	}

	return stats, nil
}
