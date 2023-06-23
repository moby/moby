// Package platform provides helper function to get the runtime architecture
// for different platforms.
package platform // import "github.com/docker/docker/pkg/platform"

import (
	"context"

	"github.com/containerd/containerd/log"
)

// Architecture holds the runtime architecture of the process.
//
// Unlike [runtime.GOARCH] (which refers to the compiler platform),
// Architecture refers to the running platform.
//
// For example, Architecture reports "x86_64" as architecture, even
// when running a "linux/386" compiled binary on "linux/amd64" hardware.
var Architecture string

func init() {
	var err error
	Architecture, err = runtimeArchitecture()
	if err != nil {
		log.G(context.TODO()).WithError(err).Error("Could not read system architecture info")
	}
}
