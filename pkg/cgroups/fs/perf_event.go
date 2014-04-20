package fs

import (
	"github.com/dotcloud/docker/pkg/cgroups"
)

type perfEventGroup struct {
}

func (s *perfEventGroup) Set(d *data) error {
	// we just want to join this group even though we don't set anything
	if _, err := d.join("perf_event"); err != nil && err != cgroups.ErrNotFound {
		return err
	}
	return nil
}

func (s *perfEventGroup) Remove(d *data) error {
	return removePath(d.path("perf_event"))
}

func (s *perfEventGroup) Stats(d *data) (map[string]float64, error) {
	return nil, ErrNotSupportStat
}
