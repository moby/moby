//go:build windows && go1.26

package sequential

import (
	"os"

	"golang.org/x/sys/windows"
)

func openFileSequential(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag|windows.O_FILE_FLAG_SEQUENTIAL_SCAN, perm)
}
