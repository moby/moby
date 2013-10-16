package docker

import (
	"github.com/dotcloud/docker/utils"
	"runtime"
	"testing"
)

func displayFdGoroutines(t *testing.T) {
	t.Logf("Fds: %d, Goroutines: %d", utils.GetTotalUsedFds(), runtime.NumGoroutine())
}

func TestFinal(t *testing.T) {
	nuke(globalRuntime)
	t.Logf("Start Fds: %d, Start Goroutines: %d", startFds, startGoroutines)
	cleanupDevMapper()
	displayFdGoroutines(t)
}
