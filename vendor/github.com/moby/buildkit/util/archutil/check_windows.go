//go:build windows
// +build windows

package archutil

import (
	"errors"
)

func check(_, _ string) (string, error) {
	return "", errors.New("binfmt is not supported on Windows")
}
