package image

import "github.com/docker/docker/api/types/container"

// Inspect contains response of Engine API:
// GET "/images/{name:.*}/json"
type Inspect struct {
	ID              string `json:"Id"`
	RepoTags        []string
	RepoDigests     []string
	Parent          string
	Comment         string
	Created         string
	Container       string
	ContainerConfig *container.Config
	DockerVersion   string
	Author          string
	Config          *container.Config
	Architecture    string
	Os              string
	OsVersion       string `json:",omitempty"`
	Size            int64
	VirtualSize     int64
	GraphDriver     container.GraphDriverData
	RootFS          RootFS
	Metadata        Metadata
}

// RootFS returns Image's RootFS description including the layer IDs.
type RootFS struct {
	Type      string
	Layers    []string `json:",omitempty"`
	BaseLayer string   `json:",omitempty"`
}
