// Package platform provides helper function to get the runtime architecture
// for different platforms.
//
// Deprecated: this package is only used internally, and will be removed in the next release.
package platform // import "github.com/docker/docker/pkg/platform"

import (
	"github.com/docker/docker/internal/platform"
)

// Architecture holds the runtime architecture of the process.
//
// Unlike [runtime.GOARCH] (which refers to the compiler platform),
// Architecture refers to the running platform.
//
// For example, Architecture reports "x86_64" as architecture, even
// when running a "linux/386" compiled binary on "linux/amd64" hardware.
//
// Deprecated: this package is only used internally, and will be removed in the next release.
var Architecture = platform.Architecture()

// NumProcs returns the number of processors on the system
//
// Deprecated: this package is only used internally, and will be removed in the next release.
func NumProcs() uint32 {
	return platform.NumProcs()
}
