package docker

import (
	"github.com/dotcloud/docker/utils"
	"os"
	"path"
	"runtime"
	"testing"
)

func displayFdGoroutines(t *testing.T) {
	t.Logf("Fds: %d, Goroutines: %d", utils.GetTotalUsedFds(), runtime.NumGoroutine())
}

func TestFinal(t *testing.T) {
	cleanup(globalRuntime)
	t.Logf("Start Fds: %d, Start Goroutines: %d", startFds, startGoroutines)
	displayFdGoroutines(t)

	if testDaemonProto == "unix" {
		os.RemoveAll(testDaemonAddr)
		os.RemoveAll(path.Dir(testDaemonAddr))
	}
}
