package main

import (
	"fmt"
	"os"

	"github.com/docker/docker/pkg/signal"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

// Copied over from docker/daemon/debugtrap_windows.go
func setupDumpStackTrap() {
	go func() {
		sa := windows.SecurityAttributes{
			Length: 0,
		}
		ev, _ := windows.UTF16PtrFromString("Global\\docker-daemon-" + fmt.Sprint(os.Getpid()))
		if h, _ := windows.CreateEvent(&sa, 0, 0, ev); h != 0 {
			logrus.Debugf("Stackdump - waiting signal at %s", ev)
			for {
				windows.WaitForSingleObject(h, windows.INFINITE)
				signal.DumpStacks("")
			}
		}
	}()
}
