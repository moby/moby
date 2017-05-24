package mountpoint

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
)

// Type represents the type of a mount.
type Type string

// Type constants
const (
	// TypeBind is the type for mounting host dir
	TypeBind Type = "bind"
	// TypeVolume is the type for remote storage volumes
	TypeVolume Type = "volume"
)

const (
	// MountPointAPIProperties is the url for plugin properties queries
	MountPointAPIProperties = "MountPointPlugin.MountPointProperties"

	// MountPointAPIAttach is the url for mount point attachment interposition
	MountPointAPIAttach = "MountPointPlugin.MountPointAttach"

	// MountPointAPIDetach is the url for mount point detachment interposition
	MountPointAPIDetach = "MountPointPlugin.MountPointDetach"

	// MountPointAPIImplements is the name of the interface all mount point plugins implement
	MountPointAPIImplements = "mountpoint"
)

// PropertiesRequest holds a mount point middleware properties query
type PropertiesRequest struct {
}

// PropertiesResponse holds some static properties of the middleware
type PropertiesResponse struct {
	// Success indicates whether the properties query was successful
	Success bool

	// Patterns is the DNF pattern set for which this middleware receives
	// interposition requests
	Patterns []Pattern

	// Err stores a message in case there's an error
	Err string `json:",omitempty"`
}

// AttachRequest holds data required for mount point middleware attachment interposition
type AttachRequest struct {
	ID     string
	Mounts []*MountPoint
}

// AttachResponse represents mount point middleware response
type AttachResponse struct {
	// Success indicates whether the mount point was successful
	Success bool

	// Attachments contains information about the middlware's participation with the mount
	Attachments []Attachment `json:",omitempty"`

	// Err stores a message in case there's an error
	Err string `json:",omitempty"`
}

// Attachment describes how the middleware will interact with the mount
type Attachment struct {
	Attach          bool
	EmergencyDetach []EmergencyDetachAction `json:",omitempty"`
	Changes         types.MountPointChanges `json:",omitempty"`
}

// EmergencyDetachAction is an action that the daemon will take on the
// plugin's behalf if the plugin is unreachable at detach time
type EmergencyDetachAction struct {
	Error   string `josn:",omitempty"`
	Fatal   string `json:",omitempty"`
	Remove  string `json:",omitempty"`
	Unmount string `json:",omitempty"`
	Warning string `json:",omitempty"`
}

// DetachRequest holds data required for mount point middleware detachment interposition
type DetachRequest struct {
	ID string
}

// DetachResponse represents mount point middleware response
type DetachResponse struct {
	// Success indicates whether detaching the mount point was successful
	Success bool

	// Recoverable indicates whether the failure (if any) is fatal to detach unwinding (false, default) or merely a container failure (true)
	Recoverable bool `json:",omitempty"`

	// Err stores a message in case there's an error
	Err string `json:",omitempty"`
}

// MountPoint is the representation of a container mount point exposed
// to mount point middleware. Pattern and Changes should be the same
// shape as this type.
type MountPoint struct {
	EffectiveSource      string
	EffectiveConsistency mount.Consistency `json:",omitempty"`

	Volume    Volume    `json:",omitempty"`
	Container Container `json:",omitempty"`
	Image     Image     `json:",omitempty"`
	// from volume/volume#MountPoint
	Source                string
	Destination           string
	ReadOnly              bool              `json:",omitempty"`
	Type                  Type              `json:",omitempty"`
	Mode                  string            `json:",omitempty"`
	Propagation           mount.Propagation `json:",omitempty"`
	CreateSourceIfMissing bool              `json:",omitempty"`

	AppliedMiddleware []types.MountPointAppliedMiddleware

	// from api/types/mount
	Consistency mount.Consistency `json:",omitempty"`
}

// Volume describes a mounted volume
type Volume struct {
	Name          string            `json:",omitempty"`
	Driver        string            `json:",omitempty"`
	ID            string            `json:",omitempty"`
	Labels        map[string]string `json:",omitempty"`
	DriverOptions map[string]string `json:",omitempty"`
	Scope         Scope             `json:",omitempty"`

	// from DetailedVolume cast
	Options map[string]string `json:",omitempty"`
}

// Container describes a mount point plugin's view of a container object
type Container struct {
	// from api/types/container.Config
	Labels map[string]string `json:",omitempty"`
}

// Image describes a mount point plugin's view of a container image
type Image struct {
	// from api/types/image_summary
	Labels map[string]string `json:",omitempty"`
}

// Scope describes the accessibility of a volume
type Scope string

// Scopes define if a volume has is cluster-wide (global) or local only.
// Scopes are returned by the volume driver when it is queried for capabilities and then set on a volume
const (
	LocalScope  Scope = "local"
	GlobalScope Scope = "global"
)

// Pattern is a description of a class of MountPoints
type Pattern struct {
	EffectiveSource      []StringPattern    `json:",omitempty"`
	EffectiveConsistency *mount.Consistency `json:",omitempty"`

	Volume    VolumePattern    `json:",omitempty"`
	Container ContainerPattern `json:",omitempty"`
	Image     ImagePattern     `json:",omitempty"`
	// from volume/volume#MountPoint
	Source                []StringPattern    `json:",omitempty"`
	Destination           []StringPattern    `json:",omitempty"`
	ReadOnly              *bool              `json:",omitempty"`
	Type                  *Type              `json:",omitempty"`
	Mode                  []StringPattern    `json:",omitempty"`
	Propagation           *mount.Propagation `json:",omitempty"`
	CreateSourceIfMissing *bool              `json:",omitempty"`

	AppliedMiddleware AppliedMiddlewareStackPattern

	// from api/types/mount
	Consistency *mount.Consistency `json:",omitempty"`
}

// VolumePattern is a description of a class of volumes
type VolumePattern struct {
	Name          []StringPattern    `json:",omitempty"`
	Driver        []StringPattern    `json:",omitempty"`
	ID            []StringPattern    `json:",omitempty"`
	Labels        []StringMapPattern `json:",omitempty"`
	DriverOptions []StringMapPattern `json:",omitempty"`
	Scope         *Scope             `json:",omitempty"`

	Options []StringMapPattern `json:",omitempty"`
}

// ContainerPattern is a description of a class of MountPoint Containers
type ContainerPattern struct {
	Labels []StringMapPattern `json:",omitempty"`
}

// ImagePattern is a description of a class of MountPoint Images
type ImagePattern struct {
	Labels []StringMapPattern `json:",omitempty"`
}

// AppliedMiddlewareStackPattern is a description of a class of
// applied middleware stack
type AppliedMiddlewareStackPattern struct {
	Exists            []AppliedMiddlewarePattern `json:",omitempty"`
	NotExists         []AppliedMiddlewarePattern `json:",omitempty"`
	All               []AppliedMiddlewarePattern `json:",omitempty"`
	NotAll            []AppliedMiddlewarePattern `json:",omitempty"`
	AnySequence       []AppliedMiddlewarePattern `json:",omitempty"`
	NotAnySequence    []AppliedMiddlewarePattern `json:",omitempty"`
	TopSequence       []AppliedMiddlewarePattern `json:",omitempty"`
	NotTopSequence    []AppliedMiddlewarePattern `json:",omitempty"`
	BottomSequence    []AppliedMiddlewarePattern `json:",omitempty"`
	NotBottomSequence []AppliedMiddlewarePattern `json:",omitempty"`
	RelativeOrder     []AppliedMiddlewarePattern `json:",omitempty"`
	NotRelativeOrder  []AppliedMiddlewarePattern `json:",omitempty"`
}

// AppliedMiddlewarePattern is a description of a class of applied middleware
type AppliedMiddlewarePattern struct {
	Name    []StringPattern `json:",omitempty"`
	Changes ChangesPattern  `json:",omitempty"`
}

// ChangesPattern is a description of a class of mount point changes
type ChangesPattern struct {
	EffectiveSource      []StringPattern    `json:",omitempty"`
	EffectiveConsistency *mount.Consistency `json:",omitempty"`

	// from api/types/mount
	//Labels      map[string]string `json:",omitempty"`
}

// StringMapPattern is a description of a class of string -> string maps
type StringMapPattern struct {
	Not bool `json:",omitempty"`

	Exists []StringMapKeyValuePattern `json:",omitempty"`
	All    []StringMapKeyValuePattern `json:",omitempty"`
}

// StringMapKeyValuePattern is a description of a class of string ->
// string map key-value pairs
type StringMapKeyValuePattern struct {
	Key   StringPattern `json:",omitempty"`
	Value StringPattern `json:",omitempty"`
}

// StringPattern is a description of a class of strings
type StringPattern struct {
	Not bool `json:",omitempty"`

	Empty        bool   `json:",omitempty"`
	Prefix       string `json:",omitempty"`
	PathPrefix   string `json:",omitempty"`
	Suffix       string `json:",omitempty"`
	PathContains string `json:",omitempty"`
	Exactly      string `json:",omitempty"`
	Contains     string `json:",omitempty"`
}
