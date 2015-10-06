package fs

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type NameGroup struct {
}

func (s *NameGroup) Apply(d *data) error {
	return nil
}

func (s *NameGroup) Set(path string, cgroup *configs.Cgroup) error {
	return nil
}

func (s *NameGroup) Remove(d *data) error {
	return nil
}

func (s *NameGroup) GetStats(path string, stats *cgroups.Stats) error {
	return nil
}
