package sysinit

import (
	"fmt"
	"os"
	"runtime"
)

// Sys Init code
// This code is run INSIDE the container and is responsible for setting
// up the environment before running the actual process
func SysInit() {
	// The very first thing that we should do is lock the thread so that other
	// system level options will work and not have issues, i.e. setns
	runtime.LockOSThread()

	if len(os.Args) <= 1 {
		fmt.Println("You should not invoke dockerinit manually")
		os.Exit(1)
	}

}
