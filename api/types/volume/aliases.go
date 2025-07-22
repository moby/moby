package volume

import "github.com/moby/moby/api/types/volume"

// ClusterVolume contains options and information specific to, and only present
// on, Swarm CSI cluster volumes.
type ClusterVolume = volume.ClusterVolume

// ClusterVolumeSpec contains the spec used to create this volume.
type ClusterVolumeSpec = volume.ClusterVolumeSpec

// Availability specifies the availability of the volume.
type Availability = volume.Availability

const (
	// AvailabilityActive indicates that the volume is active and fully
	// schedulable on the cluster.
	AvailabilityActive = volume.AvailabilityActive

	// AvailabilityPause indicates that no new workloads should use the
	// volume, but existing workloads can continue to use it.
	AvailabilityPause = volume.AvailabilityPause

	// AvailabilityDrain indicates that all workloads using this volume
	// should be rescheduled, and the volume unpublished from all nodes.
	AvailabilityDrain = volume.AvailabilityDrain
)

// AccessMode defines the access mode of a volume.
type AccessMode = volume.AccessMode

// Scope defines the Scope of a Cluster Volume. This is how many nodes a
// Volume can be accessed simultaneously on.
type Scope = volume.Scope

const (
	// ScopeSingleNode indicates the volume can be used on one node at a
	// time.
	ScopeSingleNode = volume.ScopeSingleNode

	// ScopeMultiNode indicates the volume can be used on many nodes at
	// the same time.
	ScopeMultiNode = volume.ScopeMultiNode
)

// SharingMode defines the Sharing of a Cluster Volume. This is how Tasks using a
// Volume at the same time can use it.
type SharingMode = volume.SharingMode

const (
	// SharingNone indicates that only one Task may use the Volume at a
	// time.
	SharingNone = volume.SharingNone

	// SharingReadOnly indicates that the Volume may be shared by any
	// number of Tasks, but they must be read-only.
	SharingReadOnly = volume.SharingReadOnly

	// SharingOneWriter indicates that the Volume may be shared by any
	// number of Tasks, but all after the first must be read-only.
	SharingOneWriter = volume.SharingOneWriter

	// SharingAll means that the Volume may be shared by any number of
	// Tasks, as readers or writers.
	SharingAll = volume.SharingAll
)

// TypeBlock defines options for using a volume as a block-type volume.
//
// Intentionally empty.
type TypeBlock = volume.TypeBlock

// TypeMount contains options for using a volume as a Mount-type
// volume.
type TypeMount = volume.TypeMount

// TopologyRequirement expresses the user's requirements for a volume's
// accessible topology.
type TopologyRequirement = volume.TopologyRequirement

// Topology is a map of topological domains to topological segments.
type Topology = volume.Topology

// CapacityRange describes the minimum and maximum capacity a volume should be
// created with
type CapacityRange = volume.CapacityRange

// Secret represents a Swarm Secret value that must be passed to the CSI
// storage plugin when operating on this Volume. It represents one key-value
// pair of possibly many.
type Secret = volume.Secret

// PublishState represents the state of a Volume as it pertains to its
// use on a particular Node.
type PublishState = volume.PublishState

const (
	// StatePending indicates that the volume should be published on
	// this node, but the call to ControllerPublishVolume has not been
	// successfully completed yet and the result recorded by swarmkit.
	StatePending = volume.StatePending

	// StatePublished means the volume is published successfully to the node.
	StatePublished = volume.StatePublished
	// StatePendingNodeUnpublish indicates that the Volume should be
	// unpublished on the Node, and we're waiting for confirmation that it has
	// done so.  After the Node has confirmed that the Volume has been
	// unpublished, the state will move to StatePendingUnpublish.
	StatePendingNodeUnpublish = volume.StatePendingNodeUnpublish

	// StatePendingUnpublish means the volume is still published to the node
	// by the controller, awaiting the operation to unpublish it.
	StatePendingUnpublish = volume.StatePendingUnpublish
)

// PublishStatus represents the status of the volume as published to an
// individual node
type PublishStatus = volume.PublishStatus

// Info contains information about the Volume as a whole as provided by
// the CSI storage plugin.
type Info = volume.Info

// CreateOptions VolumeConfig
//
// Volume configuration
type CreateOptions = volume.CreateOptions

// DiskUsage contains disk usage for volumes.
type DiskUsage = volume.DiskUsage

// ListResponse VolumeListResponse
//
// Volume list response
type ListResponse = volume.ListResponse

// ListOptions holds parameters to list volumes.
type ListOptions = volume.ListOptions

// PruneReport contains the response for Engine API:
// POST "/volumes/prune"
type PruneReport = volume.PruneReport

// Volume volume
type Volume = volume.Volume

// UsageData Usage details about the volume. This information is used by the
// `GET /system/df` endpoint, and omitted in other endpoints.
type UsageData = volume.UsageData

// UpdateOptions is configuration to update a Volume with.
type UpdateOptions = volume.UpdateOptions
