//go:build !windows
// +build !windows

package mounts // import "github.com/docker/docker/volume/mounts"

var (
	testDestinationPath = "/foo"
	testSourcePath      = "/foo"
)
