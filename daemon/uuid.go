package daemon // import "github.com/docker/docker/daemon"

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/pborman/uuid"
)

func loadOrCreateUUID(path string) (string, error) {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return "", err
	}
	id, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		id = []byte(uuid.New())
		if err := ioutils.AtomicWriteFile(path, id, os.FileMode(0600)); err != nil {
			return "", fmt.Errorf("Error saving uuid file: %s", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("Error loading uuid file %s: %s", path, err)
	}
	return string(id), nil
}
