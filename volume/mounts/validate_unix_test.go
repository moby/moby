//go:build !windows
// +build !windows

package mounts // import "github.com/moby/moby/volume/mounts"

var (
	testDestinationPath = "/foo"
	testSourcePath      = "/foo"
)
