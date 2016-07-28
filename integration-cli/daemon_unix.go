// +build !windows

package main

import "syscall"

func signalDaemonDump(pid int) {
	syscall.Kill(pid, syscall.SIGQUIT)
}

func signalDaemonReload(pid int) error {
	return syscall.Kill(pid, syscall.SIGHUP)
}
