// Package platform provides helper function to get the runtime architecture
// for different platforms.
package platform // import "github.com/docker/docker/pkg/platform"

import (
	"runtime"

	"github.com/sirupsen/logrus"
)

// Architecture holds the runtime architecture of the process.
//
// Unlike [runtime.GOARCH] (which refers to the compiler platform),
// Architecture refers to the running platform.
//
// For example, Architecture reports "x86_64" as architecture, even
// when running a "linux/386" compiled binary on "linux/amd64" hardware.
var Architecture string

// OSType holds the runtime operating system type of the process. It is
// an alias for [runtime.GOOS].
//
// Deprecated: use [runtime.GOOS] instead.
const OSType = runtime.GOOS

func init() {
	var err error
	Architecture, err = runtimeArchitecture()
	if err != nil {
		logrus.WithError(err).Error("Could not read system architecture info")
	}
}
