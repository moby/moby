package system

import "os"

// LCOWSupported determines if Linux Containers on Windows are supported.
// Note: This feature is in development (06/17) and enabled through an
// environment variable. At a future time, it will be enabled based
// on build number. @jhowardmsft
var lcowSupported = false

// InitLCOW sets whether LCOW is supported or not
func InitLCOW(experimental bool) {
	// LCOW initialization
	if experimental && os.Getenv("LCOW_SUPPORTED") != "" {
		lcowSupported = true
	}

}
