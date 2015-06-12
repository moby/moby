package daemon

import (
	"fmt"
	"os"

	"github.com/docker/docker/volume"
	"github.com/docker/libcontainer/label"
)

type mountPointExported struct {
	Name        string `json:",omitempty"`
	Driver      string `json:",omitempty"`
	Source      string `json:",omitempty"`
	Mode        string `json:",omitempty"`
	Destination string
}

type mountPoint struct {
	Name        string
	Destination string
	Driver      string
	RW          bool
	Volume      volume.Volume `json:"-"`
	Source      string
	Relabel     string
}

func (m *mountPoint) Setup() (string, error) {
	if m.Volume != nil {
		return m.Volume.Mount()
	}

	if len(m.Source) > 0 {
		if _, err := os.Stat(m.Source); err != nil {
			if !os.IsNotExist(err) {
				return "", err
			}
			if err := os.MkdirAll(m.Source, 0755); err != nil {
				return "", err
			}
		}
		return m.Source, nil
	}

	return "", fmt.Errorf("Unable to setup mount point, neither source nor volume defined")
}

func (m *mountPoint) Path() string {
	if m.Volume != nil {
		return m.Volume.Path()
	}

	return m.Source
}

func (m *mountPoint) createVolume() error {
	v, err := createVolume(m.Name, m.Driver)
	if err != nil {
		return err
	}
	m.Volume = v
	m.Source = v.Path()
	// Since this is just a named volume and not a typical bind, set to shared mode `z`
	if m.Relabel == "" {
		m.Relabel = "z"
	}

	return nil
}

func (m *mountPoint) isBindMount() bool {
	return len(m.Name) == 0 || len(m.Driver) == 0
}

func (m *mountPoint) applyLabel(containerLabel string) error {
	return label.Relabel(m.Source, containerLabel, m.Relabel)
}

// BackwardsCompatible decides whether this mount point can be
// used in old versions of Docker or not.
// Only bind mounts and local volumes can be used in old versions of Docker.
func (m *mountPoint) backwardsCompatible() bool {
	return len(m.Source) > 0 || m.Driver == volume.DefaultDriverName
}
