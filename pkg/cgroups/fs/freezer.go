package fs

import (
	"github.com/dotcloud/docker/pkg/cgroups"
)

type freezerGroup struct {
}

func (s *freezerGroup) Set(d *data) error {
	// we just want to join this group even though we don't set anything
	if _, err := d.join("freezer"); err != nil && err != cgroups.ErrNotFound {
		return err
	}
	return nil
}

func (s *freezerGroup) Remove(d *data) error {
	return removePath(d.path("freezer"))
}

func (s *freezerGroup) Stats(d *data) (map[string]float64, error) {
	return nil, ErrNotSupportStat
}
