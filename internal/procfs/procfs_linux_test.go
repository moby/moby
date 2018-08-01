package procfs

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"gotest.tools/assert"
)

func TestPidOf(t *testing.T) {
	pids, err := PidOf(filepath.Base(os.Args[0]))
	assert.NilError(t, err)
	assert.Check(t, len(pids) == 1)
	assert.DeepEqual(t, pids[0], os.Getpid())
}

func BenchmarkGetPids(b *testing.B) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		b.Skipf("not supported on GOOS=%s", runtime.GOOS)
	}

	re, err := regexp.Compile("(^|/)" + filepath.Base(os.Args[0]) + "$")
	assert.Check(b, err == nil)

	for i := 0; i < b.N; i++ {
		pids := getPids(re)

		b.StopTimer()
		assert.Check(b, len(pids) > 0)
		assert.Check(b, pids[0] == os.Getpid())
		b.StartTimer()
	}
}
