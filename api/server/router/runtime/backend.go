package runtime

import (
	"github.com/docker/docker/api/types/runtime"
)

// Backend is all the methods that need to be implemented by
// a runtime manager
type Backend interface {
	ListRuntimes() (runtime.GetRuntimesResponse, error)
	SetDefaultRuntime(runtime string) error
}
