package docker

import (
	"runtime"
	"testing"

	"github.com/docker/docker/pkg/fileutils"
)

func displayFdGoroutines(t *testing.T) {
	t.Logf("File Descriptors: %d, Goroutines: %d", fileutils.GetTotalUsedFds(), runtime.NumGoroutine())
}

func TestFinal(t *testing.T) {
	nuke(globalDaemon)
	t.Logf("Start File Descriptors: %d, Start Goroutines: %d", startFds, startGoroutines)
	displayFdGoroutines(t)
}
