package mount

import "github.com/moby/moby/api/types/mount"

// Type represents the type of a mount.
type Type = mount.Type

// Type constants
const (
	// TypeBind is the type for mounting host dir
	TypeBind = mount.TypeBind
	// TypeVolume is the type for remote storage volumes
	TypeVolume = mount.TypeVolume
	// TypeTmpfs is the type for mounting tmpfs
	TypeTmpfs = mount.TypeTmpfs
	// TypeNamedPipe is the type for mounting Windows named pipes
	TypeNamedPipe = mount.TypeNamedPipe
	// TypeCluster is the type for Swarm Cluster Volumes.
	TypeCluster = mount.TypeCluster
	// TypeImage is the type for mounting another image's filesystem
	TypeImage = mount.TypeImage
)

// Mount represents a mount (volume).
type Mount = mount.Mount

// Propagation represents the propagation of a mount.
type Propagation = mount.Propagation

const (
	// PropagationRPrivate RPRIVATE
	PropagationRPrivate = mount.PropagationRPrivate
	// PropagationPrivate PRIVATE
	PropagationPrivate = mount.PropagationPrivate
	// PropagationRShared RSHARED
	PropagationRShared = mount.PropagationRShared
	// PropagationShared SHARED
	PropagationShared = mount.PropagationShared
	// PropagationRSlave RSLAVE
	PropagationRSlave = mount.PropagationRSlave
	// PropagationSlave SLAVE
	PropagationSlave = mount.PropagationSlave
)

// Propagations is the list of all valid mount propagations
var Propagations = mount.Propagations

// Consistency represents the consistency requirements of a mount.
type Consistency = mount.Consistency

const (
	// ConsistencyFull guarantees bind mount-like consistency
	ConsistencyFull = mount.ConsistencyFull
	// ConsistencyCached mounts can cache read data and FS structure
	ConsistencyCached = mount.ConsistencyCached
	// ConsistencyDelegated mounts can cache read and written data and structure
	ConsistencyDelegated = mount.ConsistencyDelegated
	// ConsistencyDefault provides "consistent" behavior unless overridden
	ConsistencyDefault = mount.ConsistencyDefault
)

// BindOptions defines options specific to mounts of type "bind".
type BindOptions = mount.BindOptions

// VolumeOptions represents the options for a mount of type volume.
type VolumeOptions = mount.VolumeOptions

type ImageOptions = mount.ImageOptions

// Driver represents a volume driver.
type Driver = mount.Driver

// TmpfsOptions defines options specific to mounts of type "tmpfs".
type TmpfsOptions = mount.TmpfsOptions

// ClusterOptions specifies options for a Cluster volume.
type ClusterOptions = mount.ClusterOptions
