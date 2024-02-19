//go:build windows
// +build windows

package archutil

import (
	"errors"
)

func check(arch, bin string) (string, error) {
	return "", errors.New("binfmt is not supported on Windows")
}
