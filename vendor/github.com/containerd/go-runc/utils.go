package runc

import (
	"io/ioutil"
	"strconv"
	"syscall"
)

// ReadPidFile reads the pid file at the provided path and returns
// the pid or an error if the read and conversion is unsuccessful
func ReadPidFile(path string) (int, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(string(data))
}

const exitSignalOffset = 128

// exitStatus returns the correct exit status for a process based on if it
// was signaled or exited cleanly
func exitStatus(status syscall.WaitStatus) int {
	if status.Signaled() {
		return exitSignalOffset + int(status.Signal())
	}
	return status.ExitStatus()
}
