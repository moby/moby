package timeout

import (
	"os"
	"strconv"
	"time"
)

// Duration is the default time to wait for various operations.
// - Waiting for async notifications from HCS
// - Waiting for processes to launch through
// - Waiting to copy data to/from a launched processes stdio pipes.
//
// This can be overridden through environment variable `HCS_TIMEOUT_SECONDS`

var Duration = 4 * time.Minute

func init() {
	envTimeout := os.Getenv("HCSSHIM_TIMEOUT_SECONDS")
	if len(envTimeout) > 0 {
		e, err := strconv.Atoi(envTimeout)
		if err == nil && e > 0 {
			Duration = time.Second * time.Duration(e)
		}
	}
}
