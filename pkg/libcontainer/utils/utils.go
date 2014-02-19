package utils

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"syscall"
)

func WaitOnPid(pid int) (exitcode int, err error) {
	child, err := os.FindProcess(pid)
	if err != nil {
		return -1, err
	}
	state, err := child.Wait()
	if err != nil {
		return -1, err
	}
	return getExitCode(state), nil
}

func getExitCode(state *os.ProcessState) int {
	return state.Sys().(syscall.WaitStatus).ExitStatus()
}

func GenerateRandomName(size int) (string, error) {
	id := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		return "", err
	}
	return hex.EncodeToString(id), nil
}
