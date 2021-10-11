//go:build !windows
// +build !windows

package container // import "github.com/docker/docker/daemon/cluster/executor/container"

const (
	testAbsPath        = "/foo"
	testAbsNonExistent = "/some-non-existing-host-path/"
)
