package fdtrace

import (
	"os"
	"sync"
)

type testingM interface {
	Run() int
}

// TestMain runs m with fd tracing enabled.
//
// The function calls [os.Exit] and does not return.
func TestMain(m testingM) {
	fds = new(sync.Map)

	ret := m.Run()

	if fs := flushFrames(); len(fs) != 0 {
		for _, f := range fs {
			onLeakFD(f)
		}
	}

	if foundLeak.Load() {
		ret = 99
	}

	os.Exit(ret)
}
