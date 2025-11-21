// Code generated from OpenAPI definition. DO NOT EDIT.

package volume

// CreateRequest Volume configuration
type CreateRequest struct {
	// The new volume's name. If not specified, Docker generates a name.
	//
	// Example: tardis
	Name string `json:"Name,omitempty"`

	// Name of the volume driver to use.
	// Example: custom
	Driver string `json:"Driver,omitempty"`

	// A mapping of driver options and values. These options are
	// passed directly to the driver and are driver specific.
	//
	// Example: {
	//   "device": "tmpfs",
	//   "o": "size=100m,uid=1000",
	//   "type": "tmpfs"
	// }
	DriverOpts map[string]string `json:"DriverOpts,omitempty"`

	// User-defined key/value metadata.
	// Example: {
	//   "com.example.some-label": "some-value",
	//   "com.example.some-other-label": "some-other-value"
	// }
	Labels map[string]string `json:"Labels,omitempty"`

	// Cluster-specific options used to create the volume.
	//
	ClusterVolumeSpec *ClusterVolumeSpec `json:"ClusterVolumeSpec,omitempty"`
}
