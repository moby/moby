package swarm

import "time"

// ContainerSpec represents the spec of a container.
type ContainerSpec struct {
	Image           string            `json:",omitempty"`
	Labels          map[string]string `json:",omitempty"`
	Command         []string          `json:",omitempty"`
	Args            []string          `json:",omitempty"`
	Env             []string          `json:",omitempty"`
	Dir             string            `json:",omitempty"`
	User            string            `json:",omitempty"`
	Mounts          []Mount           `json:",omitempty"`
	StopGracePeriod *time.Duration    `json:",omitempty"`
}

// Mount represents a mount (volume).
type Mount struct {
	Target        string              `json:",omitempty"`
	Source        string              `json:",omitempty"`
	Writable      bool                `json:",omitempty"`
	Type          MountType           `json:",omitempty"`
	Propagation   MountPropagation    `json:",omitempty"`
	MCSAccessMode MountMCSAaccessMode `json:",omitempty"`
	Populate      bool                `json:",omitempty"`
	Template      *VolumeTemplate     `json:",omitempty"`
}

const (
	// MountPropagationRPrivate RPRIVATE
	MountPropagationRPrivate = "RPRIVATE"
	// MountPropagationPrivate PRIVATE
	MountPropagationPrivate = "PRIVATE"
	// MountPropagationRShared RSHARED
	MountPropagationRShared = "RSHARED"
	// MountPropagationShared SHARED
	MountPropagationShared = "SHARED"
	// MountPropagationRSlave RSLAVE
	MountPropagationRSlave = "RSLAVE"
	// MountPropagationSlave SLAVE
	MountPropagationSlave = "SLAVE"
)

// MountPropagation represents the propagation of a mount.
type MountPropagation string

const (
	// MountMCSAccessModeNone    MCSNONE
	MountMCSAccessModeNone = "MCSNONE"
	// MountMCSAccessModeShared  MCSSHARED
	MountMCSAccessModeShared = "MCSSHARED"
	// MountMCSAccessModePrivate MCSPRIVATE
	MountMCSAccessModePrivate = "MCSPRIVATE"
)

// MountMCSAaccessMode represents the MCSAaccessMode of a mount.
type MountMCSAaccessMode string

const (
	// MountTypeBind BIND
	MountTypeBind = "BIND"
	// MountTypeVolume VOLUME
	MountTypeVolume = "VOLUME"
)

// MountType represents the type of a mount.
type MountType string

// VolumeTemplate represents the template of a volume.
type VolumeTemplate struct {
	Annotations
	DriverConfig Driver `json:",omitempty"`
}
