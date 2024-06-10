package container

import (
	"io"
	"os"
	"time"
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

// CopyToContainerOptions holds information
// about files to copy into a container
type CopyToContainerOptions struct {
	AllowOverwriteDirWithFile bool
	CopyUIDGID                bool
}

// StatsResponse contains response of Engine API:
// GET "/stats"
type StatsResponse struct {
	Body   io.ReadCloser `json:"body"`
	OSType string        `json:"ostype"`
}
