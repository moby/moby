package daemon // import "github.com/docker/docker/daemon"

import (
	"os"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/libtrust"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// loadOrCreateID loads the engine's ID from idPath, or generates a new ID
// if it doesn't exist. It returns the ID, and any error that occurred when
// saving the file.
//
// Note that this function expects the daemon's root directory to already have
// been created with the right permissions and ownership (usually this would
// be done by daemon.CreateDaemonRoot().
func loadOrCreateID(idPath string) (string, error) {
	var id string
	idb, err := os.ReadFile(idPath)
	if os.IsNotExist(err) {
		id = uuid.New().String()
		if err := ioutils.AtomicWriteFile(idPath, []byte(id), os.FileMode(0600)); err != nil {
			return "", errors.Wrap(err, "error saving ID file")
		}
	} else if err != nil {
		return "", errors.Wrapf(err, "error loading ID file %s", idPath)
	} else {
		id = string(idb)
	}
	return id, nil
}

// migrateTrustKeyID migrates the daemon ID of existing installations. It returns
// an error when a trust-key was found, but we failed to read it, or failed to
// complete the migration.
//
// We migrate the ID so that engines don't get a new ID generated on upgrades,
// which may be unexpected (and users may be using the ID for various purposes).
func migrateTrustKeyID(deprecatedTrustKeyPath, idPath string) error {
	if _, err := os.Stat(idPath); err == nil {
		// engine ID file already exists; no migration needed
		return nil
	}
	trustKey, err := libtrust.LoadKeyFile(deprecatedTrustKeyPath)
	if err != nil {
		if err == libtrust.ErrKeyFileDoesNotExist {
			// no existing trust-key found; no migration needed
			return nil
		}
		return err
	}
	id := trustKey.PublicKey().KeyID()
	if err := ioutils.AtomicWriteFile(idPath, []byte(id), os.FileMode(0600)); err != nil {
		return errors.Wrap(err, "error saving ID file")
	}
	logrus.Info("successfully migrated engine ID")
	return nil
}
