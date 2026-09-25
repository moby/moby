//go:build !windows

package appcontext

import (
	"os"

	"golang.org/x/sys/unix"
)

var terminationSignals = []os.Signal{unix.SIGTERM, unix.SIGINT}
