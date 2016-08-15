package volume

import (
	apirouter "github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/types"
	"github.com/spf13/cobra"
)

// TODO: update the old interface with what is actually used
// TODO: move types into this package
type Backend interface {
	Volumes(filter string) ([]*types.Volume, []string, error)
	VolumeInspect(name string) (*types.Volume, error)
	VolumeCreate(name, driverName string, opts, labels map[string]string) (*types.Volume, error)
	VolumeRm(name string, force bool) error
}

type VolumeComponent struct {
}

func (c *VolumeComponent) Provides() string {
	return "volumes"
}

func (c *VolumeComponent) Routes() []apirouter.Route {
	return nil
}

func (c *VolumeComponent) CommandLine() []*cobra.Command {
	return nil
}
