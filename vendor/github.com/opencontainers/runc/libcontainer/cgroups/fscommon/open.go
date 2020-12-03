package fscommon

import (
	"os"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/pkg/errors"
)

var (
	// Set to true by fs unit tests
	TestMode bool
)

// OpenFile opens a cgroup file in a given dir with given flags.
// It is supposed to be used for cgroup files only.
func OpenFile(dir, file string, flags int) (*os.File, error) {
	if dir == "" {
		return nil, errors.Errorf("no directory specified for %s", file)
	}
	mode := os.FileMode(0)
	if TestMode && flags&os.O_WRONLY != 0 {
		// "emulate" cgroup fs for unit tests
		flags |= os.O_TRUNC | os.O_CREATE
		mode = 0o600
	}
	path, err := securejoin.SecureJoin(dir, file)
	if err != nil {
		return nil, err
	}

	return os.OpenFile(path, flags, mode)
}
