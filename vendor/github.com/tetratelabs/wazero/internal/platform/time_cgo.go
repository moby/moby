//go:build cgo && !windows

package platform

import _ "unsafe" // for go:linkname

// nanotime uses runtime.nanotime as it is available on all platforms and
// benchmarks faster than using time.Since.
//
//go:linkname nanotime runtime.nanotime
func nanotime() int64
