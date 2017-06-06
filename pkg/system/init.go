package system

import (
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	// Used by chtimes
	maxTime time.Time

	// LCOWSupported determines if Linux Containers on Windows are supported.
	// Note: This feature is in development (04/17) and enabled through an
	// environment variable. At a future time, it will be enabled based
	// on build number. @jhowardmsft
	lcowSupported = false
)

func init() {
	// chtimes initialization
	if unsafe.Sizeof(syscall.Timespec{}.Nsec) == 8 {
		// This is a 64 bit timespec
		// os.Chtimes limits time to the following
		maxTime = time.Unix(0, 1<<63-1)
	} else {
		// This is a 32 bit timespec
		maxTime = time.Unix(1<<31-1, 0)
	}

	// LCOW initialization
	if runtime.GOOS == "windows" && os.Getenv("LCOW_SUPPORTED") != "" {
		lcowSupported = true
	}
}
