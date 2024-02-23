package cnmallocator

import (
	"runtime"
	"testing"

	"github.com/moby/swarmkit/v2/manager/allocator"
	"gotest.tools/v3/skip"
)

func TestAllocator(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "Allocator tests are hardcoded to use Linux network driver names")
	allocator.RunAllocatorTests(t, NewProvider(nil))
}
