package client

import (
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/filters"
)

// ContainerAttachOptions holds parameters to attach to a container.
type ContainerAttachOptions struct {
	Stream     bool
	Stdin      bool
	Stdout     bool
	Stderr     bool
	DetachKeys string
	Logs       bool
}

// ContainerCommitOptions holds parameters to commit changes into a container.
type ContainerCommitOptions struct {
	Reference string
	Comment   string
	Author    string
	Changes   []string
	Pause     bool
	Config    *container.Config
}

// ContainerRemoveOptions holds parameters to remove containers.
type ContainerRemoveOptions struct {
	RemoveVolumes bool
	RemoveLinks   bool
	Force         bool
}

// ContainerStartOptions holds parameters to start containers.
type ContainerStartOptions struct {
	CheckpointID  string
	CheckpointDir string
}

// ContainerListOptions holds parameters to list containers with.
type ContainerListOptions struct {
	Size    bool
	All     bool
	Latest  bool
	Since   string
	Before  string
	Limit   int
	Filters filters.Args
}

// ContainerLogsOptions holds parameters to filter logs with.
type ContainerLogsOptions struct {
	ShowStdout bool
	ShowStderr bool
	Since      string
	Until      string
	Timestamps bool
	Follow     bool
	Tail       string
	Details    bool
}
