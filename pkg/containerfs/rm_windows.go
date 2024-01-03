package containerfs // import "github.com/docker/docker/pkg/containerfs"

import "os"

// EnsureRemoveAll is an alias to os.RemoveAll on Windows
var EnsureRemoveAll = os.RemoveAll
