// +build !windows

package local

import "github.com/docker/docker/api/types/versions/v1p19" // container json

// InspectPre120Backend holds the definition of the container inspect
// call, responding with pre 1.20 API values. Windows has a different
// return type.
type InspectPre120Backend interface {
	ContainerInspectPre120(name string) (*v1p19.ContainerJSON, error)
}
