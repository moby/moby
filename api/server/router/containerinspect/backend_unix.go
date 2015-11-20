// +build !windows

package containerinspect

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions/v1p19"
	"github.com/docker/docker/api/types/versions/v1p20"
)

// Backend has all the methods to support container inspect functionality
type Backend interface {
	ContainerInspect(name string, size bool) (*types.ContainerJSON, error)
	ContainerInspect120(name string) (*v1p20.ContainerJSON, error)
	// unix version
	ContainerInspectPre120(name string) (*v1p19.ContainerJSON, error)
	// windows version
	//ContainerInspectPre120(name string) (*types.ContainerJSON, error)
}
