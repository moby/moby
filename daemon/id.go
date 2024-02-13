package daemon // import "github.com/docker/docker/daemon"

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const idFilename = "engine-id"

// LoadOrCreateID loads the engine's ID from the given root, or generates a new ID
// if it doesn't exist. It returns the ID, and any error that occurred when
// saving the file.
//
// Note that this function expects the daemon's root directory to already have
// been created with the right permissions and ownership (usually this would
// be done by daemon.CreateDaemonRoot().
func LoadOrCreateID(root string) (string, error) {
	var id string
	idPath := filepath.Join(root, idFilename)
	idb, err := os.ReadFile(idPath)
	if os.IsNotExist(err) {
		id = uuid.New().String()
		if err := ioutils.AtomicWriteFile(idPath, []byte(id), os.FileMode(0o600)); err != nil {
			return "", errors.Wrap(err, "error saving ID file")
		}
	} else if err != nil {
		return "", errors.Wrapf(err, "error loading ID file %s", idPath)
	} else {
		id = string(idb)
	}
	return id, nil
}
