package container

import (
	"github.com/moby/moby/api/types/storage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ContainerJSONBase contains response of Engine API GET "/containers/{name:.*}/json"
// for API version 1.18 and older.
//
// TODO(thaJeztah): combine ContainerJSONBase and InspectResponse into a single struct.
// The split between ContainerJSONBase (ContainerJSONBase) and InspectResponse (InspectResponse)
// was done in commit 6deaa58ba5f051039643cedceee97c8695e2af74 (https://github.com/moby/moby/pull/13675).
// ContainerJSONBase contained all fields for API < 1.19, and InspectResponse
// held fields that were added in API 1.19 and up. Given that the minimum
// supported API version is now 1.24, we no longer use the separate type.
type ContainerJSONBase struct {
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
	GraphDriver     storage.DriverData
	SizeRw          *int64 `json:",omitempty"`
	SizeRootFs      *int64 `json:",omitempty"`
}

// InspectResponse is the response for the GET "/containers/{name:.*}/json"
// endpoint.
type InspectResponse struct {
	*ContainerJSONBase
	Mounts          []MountPoint
	Config          *Config
	NetworkSettings *NetworkSettings
	// ImageManifestDescriptor is the descriptor of a platform-specific manifest of the image used to create the container.
	ImageManifestDescriptor *ocispec.Descriptor `json:"ImageManifestDescriptor,omitempty"`
}
