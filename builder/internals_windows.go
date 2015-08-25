// +build windows

package builder

import (
	"io/ioutil"
	"strings"
)

func getTempDir(dir, prefix string) (string, error) {

	tempDir, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(tempDir, `\\?\`) {
		tempDir = `\\?\` + tempDir
	}

	return tempDir, nil
}

func fixPermissions(source, destination string, uid, gid int, destExisted bool) error {
	// chown is not supported on Windows
	return nil
}
