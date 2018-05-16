package container

import (
	"os"
	"time"
)

// CopyConfig contains request body of Engine API:
// POST "/containers/"+containerID+"/copy"
type CopyConfig struct {
	Resource string
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
