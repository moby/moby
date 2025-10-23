package client

import (
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ContainerCreateOptions holds parameters to create a container.
type ContainerCreateOptions struct {
	Config           *container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
	Platform         *ocispec.Platform
	ContainerName    string
}

// ContainerCreateResult is the result from creating a container.
type ContainerCreateResult struct {
	ID       string
	Warnings []string
}
