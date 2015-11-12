package local

import "github.com/docker/docker/api/types" // container json

// InspectPre120Backend holds the definition of the container inspect
// call, responding with pre 1.20 API values. Windows has a different
// return type.
type InspectPre120Backend interface {
	ContainerInspectPre120(name string) (*types.ContainerJSON, error)
}
