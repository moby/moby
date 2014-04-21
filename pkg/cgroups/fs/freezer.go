package fs

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/cgroups"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	var (
		paramData = make(map[string]float64)
		params    = []string{
			"parent_freezing",
			"self_freezing",
			// comment out right now because this is string "state",
		}
	)

	path, err := d.path("freezer")
	if err != nil {
		return nil, err
	}

	for _, param := range params {
		f, err := os.Open(filepath.Join(path, fmt.Sprintf("freezer.%s", param)))
		if err != nil {
			return nil, err
		}
		defer f.Close()

		data, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}

		v, err := strconv.ParseFloat(strings.TrimSuffix(string(data), "\n"), 64)
		if err != nil {
			return nil, err
		}
		paramData[param] = v
	}
	return paramData, nil
}
