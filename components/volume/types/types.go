package types

// Volume represents the configuration of a volume for the remote API
type Volume struct {
	Name       string                 // Name is the name of the volume
	Driver     string                 // Driver is the Driver name used to create the volume
	Mountpoint string                 // Mountpoint is the location on disk of the volume
	Status     map[string]interface{} `json:",omitempty"` // Status provides low-level status information about the volume
	Labels     map[string]string      // Labels is metadata specific to the volume
	Scope      string                 // Scope describes the level at which the volume exists (e.g. `global` for cluster-wide or `local` for machine level)
}

// VolumesListResponse contains the response for the remote API:
// GET "/volumes"
type VolumesListResponse struct {
	Volumes  []*Volume // Volumes is the list of volumes being returned
	Warnings []string  // Warnings is a list of warnings that occurred when getting the list from the volume drivers
}

// VolumeCreateRequest contains the response for the remote API:
// POST "/volumes/create"
type VolumeCreateRequest struct {
	Name       string            // Name is the requested name of the volume
	Driver     string            // Driver is the name of the driver that should be used to create the volume
	DriverOpts map[string]string // DriverOpts holds the driver specific options to use for when creating the volume.
	Labels     map[string]string // Labels holds metadata specific to the volume being created.
}
