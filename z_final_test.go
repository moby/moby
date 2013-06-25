package docker

import (
	"github.com/dotcloud/docker/utils"
	"runtime"
	"testing"
)

func TestFinal(t *testing.T) {
	cleanup(globalRuntime)
	t.Logf("Fds: %d, Goroutines: %d", utils.GetTotalUsedFds(), runtime.NumGoroutine())
}
