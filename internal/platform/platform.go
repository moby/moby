package platform

import (
	"context"
	"sync"

	"github.com/containerd/log"
)

var (
	arch     string
	onceArch sync.Once
)

// Architecture returns the runtime architecture of the process.
//
// Unlike [runtime.GOARCH] (which refers to the compiler platform),
// Architecture refers to the running platform.
//
// For example, Architecture reports "x86_64" as architecture, even
// when running a "linux/386" compiled binary on "linux/amd64" hardware.
func Architecture() string {
	onceArch.Do(func() {
		var err error
		arch, err = runtimeArchitecture()
		if err != nil {
			log.G(context.TODO()).WithError(err).Error("Could not read system architecture info")
		}
	})
	return arch
}
