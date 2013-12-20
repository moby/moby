package utils

import (
	"os"
	"os/signal"
	"syscall"
)

func StopCatch(sigc chan os.Signal) {
	signal.Stop(sigc)
	close(sigc)
}

// OS specific map to signals. map[originSignal]mappedSignal
var signalMap map[syscall.Signal]syscall.Signal

// SignalMap normalize the signals to Linux standard.
func SignalMap(src syscall.Signal) syscall.Signal {
	// If the OS didn't register a map, return direcly the signal
	if signalMap == nil {
		return src
	}
	// If the signal is found, return the mapped value
	if dst, ok := signalMap[src]; ok {
		return dst
	}

	// Otherwise, return the given signal
	return src
}

// Suspend sends SIGSTOP to the calling process.
func Suspend() error {
	return suspend()
}
