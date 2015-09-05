// +build windows

package dockerfile

import (
	"io/ioutil"

	"github.com/docker/docker/pkg/longpath"
)

func getTempDir(dir, prefix string) (string, error) {
	tempDir, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		return "", err
	}
	return longpath.AddPrefix(tempDir), nil
}

func fixPermissions(source, destination string, uid, gid int, destExisted bool) error {
	// chown is not supported on Windows
	return nil
}
