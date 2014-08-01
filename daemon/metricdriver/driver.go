package metricdriver

import (
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/cgroups/fs"
)

func Get(id, parent string) (*cgroups.Stats, error) {

	c := &cgroups.Cgroup{
		Name:   id,
		Parent: parent,
	}

	stats, err := fs.GetStatsFix(c)

	if err != nil {
		return nil, err
	}

	return stats, nil
}
