// +build windows

package utils

import (
	"path/filepath"
	"strings"
)

func getContextRoot(srcPath string) (string, error) {
	cr, err := filepath.Abs(srcPath)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(cr, `\\?\`) {
		cr = `\\?\` + cr
	}
	return cr, nil
}
