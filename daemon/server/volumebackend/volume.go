package volumebackend

import (
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/v2/daemon/internal/filters"
)

// CreateOptions holds options for creating a volume.
type CreateOptions struct {
	// cluster volume spec
	ClusterVolumeSpec *volume.ClusterVolumeSpec `json:"ClusterVolumeSpec,omitempty"`

	// Name of the volume driver to use.
	// Example: custom
	Driver string `json:"Driver,omitempty"`

	// A mapping of driver options and values. These options are
	// passed directly to the driver and are driver specific.
	//
	// Example: {"device":"tmpfs","o":"size=100m,uid=1000","type":"tmpfs"}
	DriverOpts map[string]string `json:"DriverOpts,omitempty"`

	// User-defined key/value metadata.
	// Example: {"com.example.some-label":"some-value","com.example.some-other-label":"some-other-value"}
	Labels map[string]string `json:"Labels,omitempty"`

	// The new volume's name. If not specified, Docker generates a name.
	//
	// Example: tardis
	Name string `json:"Name,omitempty"`
}

type ListOptions struct {
	Filters filters.Args
}

type UpdateOptions struct {
	Spec *volume.ClusterVolumeSpec `json:"Spec,omitempty"`
}
