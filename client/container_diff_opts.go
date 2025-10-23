package client

import "github.com/moby/moby/api/types/container"

// ContainerDiffOptions holds parameters to show differences in a container filesystem.
type ContainerDiffOptions struct {
	// Currently no options, but this allows for future extensibility
}

// ContainerDiffResult is the result from showing differences in a container filesystem.
type ContainerDiffResult struct {
	Changes []container.FilesystemChange
}
