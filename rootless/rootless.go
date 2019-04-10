package rootless // import "github.com/docker/docker/rootless"

import (
	"os"
	"sync"
)

var (
	runningWithNonRootUsername     bool
	runningWithNonRootUsernameOnce sync.Once
)

// RunningWithNonRootUsername returns true if we $USER is set to a non-root value,
// regardless to the UID/EUID value.
//
// The value of this variable is mostly used for configuring default paths.
// If the value is true, $HOME and $XDG_RUNTIME_DIR should be honored for setting up the default paths.
// If false (not only EUID==0 but also $USER==root), $HOME and $XDG_RUNTIME_DIR should be ignored
// even if we are in a user namespace.
func RunningWithNonRootUsername() bool {
	runningWithNonRootUsernameOnce.Do(func() {
		u := os.Getenv("USER")
		runningWithNonRootUsername = u != "" && u != "root"
	})
	return runningWithNonRootUsername
}
