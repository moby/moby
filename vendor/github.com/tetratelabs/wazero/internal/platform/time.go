package platform

import (
	"sync/atomic"
	"time"

	"github.com/tetratelabs/wazero/sys"
)

const (
	ms = int64(time.Millisecond)
	// FakeEpochNanos is midnight UTC 2022-01-01 and exposed for testing
	FakeEpochNanos = 1640995200000 * ms
)

// NewFakeWalltime implements sys.Walltime with FakeEpochNanos that increases by 1ms each reading.
// See /RATIONALE.md
func NewFakeWalltime() sys.Walltime {
	// AddInt64 returns the new value. Adjust so the first reading will be FakeEpochNanos
	t := FakeEpochNanos - ms
	return func() (sec int64, nsec int32) {
		wt := atomic.AddInt64(&t, ms)
		return wt / 1e9, int32(wt % 1e9)
	}
}

// NewFakeNanotime implements sys.Nanotime that increases by 1ms each reading.
// See /RATIONALE.md
func NewFakeNanotime() sys.Nanotime {
	// AddInt64 returns the new value. Adjust so the first reading will be zero.
	t := int64(0) - ms
	return func() int64 {
		return atomic.AddInt64(&t, ms)
	}
}

// FakeNanosleep implements sys.Nanosleep by returning without sleeping.
var FakeNanosleep = sys.Nanosleep(func(int64) {})

// FakeOsyield implements sys.Osyield by returning without yielding.
var FakeOsyield = sys.Osyield(func() {})

// Walltime implements sys.Walltime with time.Now.
//
// Note: This is only notably less efficient than it could be is reading
// runtime.walltime(). time.Now defensively reads nanotime also, just in case
// time.Since is used. This doubles the performance impact. However, wall time
// is likely to be read less frequently than Nanotime. Also, doubling the cost
// matters less on fast platforms that can return both in <=100ns.
func Walltime() (sec int64, nsec int32) {
	t := time.Now()
	return t.Unix(), int32(t.Nanosecond())
}

// nanoBase uses time.Now to ensure a monotonic clock reading on all platforms
// via time.Since.
var nanoBase = time.Now()

// nanotimePortable implements sys.Nanotime with time.Since.
//
// Note: This is less efficient than it could be is reading runtime.nanotime(),
// Just to do that requires CGO.
func nanotimePortable() int64 {
	return time.Since(nanoBase).Nanoseconds()
}

// Nanotime implements sys.Nanotime with runtime.nanotime() if CGO is available
// and time.Since if not.
func Nanotime() int64 {
	return nanotime()
}

// Nanosleep implements sys.Nanosleep with time.Sleep.
func Nanosleep(ns int64) {
	time.Sleep(time.Duration(ns))
}
