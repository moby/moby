//go:build !windows
// +build !windows

package loggerutils

import "os"

func openFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func open(name string) (*os.File, error) {
	return os.Open(name)
}

func unlink(name string) error {
	return os.Remove(name)
}
