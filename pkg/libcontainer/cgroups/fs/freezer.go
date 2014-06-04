package fs

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/dotcloud/docker/pkg/libcontainer/cgroups"
)

type freezerGroup struct {
}

func (s *freezerGroup) Set(d *data) error {
	switch d.c.Freezer {
	case cgroups.Frozen, cgroups.Thawed:
		dir, err := d.path("freezer")
		if err != nil {
			return err
		}

		if err := writeFile(dir, "freezer.state", string(d.c.Freezer)); err != nil {
			return err
		}

		for {
			state, err := readFile(dir, "freezer.state")
			if err != nil {
				return err
			}
			if strings.TrimSpace(state) == string(d.c.Freezer) {
				break
			}
			time.Sleep(1 * time.Millisecond)
		}
	default:
		if _, err := d.join("freezer"); err != nil && err != cgroups.ErrNotFound {
			return err
		}
	}

	return nil
}

func (s *freezerGroup) Remove(d *data) error {
	return removePath(d.path("freezer"))
}

func getFreezerFileData(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	return strings.TrimSuffix(string(data), "\n"), err
}

func (s *freezerGroup) GetStats(d *data, stats *cgroups.Stats) error {
	path, err := d.path("freezer")
	if err != nil {
		return err
	}
	var data string
	if data, err = getFreezerFileData(filepath.Join(path, "freezer.parent_freezing")); err != nil {
		return err
	}
	stats.FreezerStats.ParentState = data
	if data, err = getFreezerFileData(filepath.Join(path, "freezer.self_freezing")); err != nil {
		return err
	}
	stats.FreezerStats.SelfState = data

	return nil
}
