package debug // import "github.com/docker/docker/cmd/dockerd/debug"

import (
	"os"

	"github.com/containerd/log"
)

// Enable sets the DEBUG env var to true
// and makes the logger to log at debug level.
func Enable() {
	os.Setenv("DEBUG", "1")
	_ = log.SetLevel("debug")
}

// Disable sets the DEBUG env var to false
// and makes the logger to log at info level.
func Disable() {
	os.Setenv("DEBUG", "")
	_ = log.SetLevel("info")
}

// IsEnabled checks whether the debug flag is set or not.
func IsEnabled() bool {
	return os.Getenv("DEBUG") != ""
}
