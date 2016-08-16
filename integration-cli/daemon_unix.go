// +build !windows

package main

import "syscall"

func signalDaemonDump(pid int) {
	syscall.Kill(pid, syscall.SIGQUIT)
}
