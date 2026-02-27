package container

import (
	"encoding/json"
	"os"
	"time"

	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/storage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// PruneReport contains the response for Engine API:
// POST "/containers/prune"
type PruneReport struct {
	ContainersDeleted []string
	SpaceReclaimed    uint64
}

// PathStat is used to encode the header from
// GET "/containers/{name:.*}/archive"
// "Name" is the file or directory name.
type PathStat struct {
	Name       string      `json:"name"`
	Size       int64       `json:"size"`
	Mode       os.FileMode `json:"mode"`
	Mtime      time.Time   `json:"mtime"`
	LinkTarget string      `json:"linkTarget"`
}

// MountPoint represents a mount point configuration inside the container.
// This is used for reporting the mountpoints in use by a container.
type MountPoint struct {
	// Type is the type of mount, see [mount.Type] definitions for details.
	Type mount.Type `json:",omitempty"`

	// Name is the name reference to the underlying data defined by `Source`
	// e.g., the volume name.
	Name string `json:",omitempty"`

	// Source is the source location of the mount.
	//
	// For volumes, this contains the storage location of the volume (within
	// `/var/lib/docker/volumes/`). For bind-mounts, and `npipe`, this contains
	// the source (host) part of the bind-mount. For `tmpfs` mount points, this
	// field is empty.
	Source string

	// Destination is the path relative to the container root (`/`) where the
	// Source is mounted inside the container.
	Destination string

	// Driver is the volume driver used to create the volume (if it is a volume).
	Driver string `json:",omitempty"`

	// Mode is a comma separated list of options supplied by the user when
	// creating the bind/volume mount.
	//
	// The default is platform-specific (`"z"` on Linux, empty on Windows).
	Mode string

	// RW indicates whether the mount is mounted writable (read-write).
	RW bool

	// Propagation describes how mounts are propagated from the host into the
	// mount point, and vice-versa. Refer to the Linux kernel documentation
	// for details:
	// https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt
	//
	// This field is not used on Windows.
	Propagation mount.Propagation
}

// State stores container's running state
// it's part of ContainerJSONBase and returned by "inspect" command
type State struct {
	Status     ContainerState // String representation of the container state. Can be one of "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	Running    bool
	Paused     bool
	Restarting bool
	OOMKilled  bool
	Dead       bool
	Pid        int
	ExitCode   int
	Error      string
	StartedAt  string
	FinishedAt string
	Health     *Health `json:",omitempty"`
}

// Summary contains response of Engine API:
// GET "/containers/json"
type Summary struct {
	ID                      string `json:"Id"`
	Names                   []string
	Image                   string
	ImageID                 string
	ImageManifestDescriptor *ocispec.Descriptor `json:"ImageManifestDescriptor,omitempty"`
	Command                 string
	Created                 int64
	Ports                   []PortSummary
	SizeRw                  int64 `json:",omitempty"`
	SizeRootFs              int64 `json:",omitempty"`
	Labels                  map[string]string
	State                   ContainerState
	Status                  string
	HostConfig              struct {
		NetworkMode string            `json:",omitempty"`
		Annotations map[string]string `json:",omitempty"`
	}
	Health          *HealthSummary `json:",omitempty"`
	NetworkSettings *NetworkSettingsSummary
	Mounts          []MountPoint
}

// InspectResponse is the response for the GET "/containers/{name:.*}/json"
// endpoint.
type InspectResponse struct {
	ID              string `json:"Id"`
	Created         string
	Path            string
	Args            []string
	State           *State
	Image           string
	ResolvConfPath  string
	HostnamePath    string
	HostsPath       string
	LogPath         string
	Name            string
	RestartCount    int
	Driver          string
	Platform        string
	MountLabel      string
	ProcessLabel    string
	AppArmorProfile string
	ExecIDs         []string
	HostConfig      *HostConfig

	// GraphDriver contains information about the container's graph driver.
	GraphDriver *storage.DriverData `json:"GraphDriver,omitempty"`

	// Storage contains information about the storage used for the container's filesystem.
	Storage *storage.Storage `json:"Storage,omitempty"`

	SizeRw          *int64 `json:",omitempty"`
	SizeRootFs      *int64 `json:",omitempty"`
	Mounts          []MountPoint
	Config          *Config
	NetworkSettings *NetworkSettings
	// ImageManifestDescriptor is the descriptor of a platform-specific manifest of the image used to create the container.
	ImageManifestDescriptor *ocispec.Descriptor `json:"ImageManifestDescriptor,omitempty"`

	Spec map[string]json.RawMessage `json:",omitempty"`
}
