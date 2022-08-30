package system

import (
	"os"

	"github.com/moby/sys/sequential"
)

// CreateSequential is deprecated.
//
// Deprecated: use os.Create or github.com/moby/sys/sequential.Create()
func CreateSequential(name string) (*os.File, error) {
	return sequential.Create(name)
}

// OpenSequential is deprecated.
//
// Deprecated: use os.Open or github.com/moby/sys/sequential.Open
func OpenSequential(name string) (*os.File, error) {
	return sequential.Open(name)
}

// OpenFileSequential is deprecated.
//
// Deprecated: use github.com/moby/sys/sequential.OpenFile()
func OpenFileSequential(name string, flag int, perm os.FileMode) (*os.File, error) {
	return sequential.OpenFile(name, flag, perm)
}

// TempFileSequential is deprecated.
//
// Deprecated: use os.CreateTemp or github.com/moby/sys/sequential.CreateTemp
func TempFileSequential(dir, prefix string) (f *os.File, err error) {
	return sequential.CreateTemp(dir, prefix)
}
