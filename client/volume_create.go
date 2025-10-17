package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/volume"
)

// VolumeCreateOptions holds parameters to create a volume.
type VolumeCreateOptions struct {
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

type VolumeCreateResult struct {
	volume.Volume
}

// VolumeCreate creates a volume in the docker host.
func (cli *Client) VolumeCreate(ctx context.Context, options VolumeCreateOptions) (VolumeCreateResult, error) {
	resp, err := cli.post(ctx, "/volumes/create", nil, options, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return VolumeCreateResult{}, err
	}

	var vol volume.Volume
	err = json.NewDecoder(resp.Body).Decode(&vol)
	return VolumeCreateResult{Volume: vol}, err
}
