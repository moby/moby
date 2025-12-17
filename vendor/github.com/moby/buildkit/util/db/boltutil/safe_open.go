package boltutil

import (
	"os"

	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/db"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

// SafeOpen opens a bolt database with automatic recovery from corruption.
// If the database file is corrupted, it backs up the corrupted file and creates
// a new empty database. This is useful for disposable databases like cache or
// history where data loss is acceptable but startup failure is not.
func SafeOpen(dbPath string, mode os.FileMode, opts *bolt.Options) (db db.DB, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("%v", r)
		}

		// If we get an error when opening the database, but we have
		// access to the file and the file looks like it has content,
		// then fallback to resetting the database since the database
		// may be corrupt.
		if err != nil && fileHasContent(dbPath) {
			db, err = fallbackOpen(dbPath, mode, opts, err)
		}
	}()
	return Open(dbPath, mode, opts)
}

// fallbackOpen performs database recovery and opens a new database
// file when the database fails to open. Called after the first database
// open fails.
func fallbackOpen(dbPath string, mode os.FileMode, opts *bolt.Options, openErr error) (db.DB, error) {
	backupPath := dbPath + "." + identity.NewID() + ".bak"
	bklog.L.Errorf("failed to open database file %s, resetting to empty. Old database is backed up to %s. "+
		"This error signifies that buildkitd likely crashed or was sigkilled abruptly, leaving the database corrupted. "+
		"If you see logs from a previous panic then please report in the issue tracker at https://github.com/moby/buildkit . %+v", dbPath, backupPath, openErr)
	if err := os.Rename(dbPath, backupPath); err != nil {
		return nil, errors.Wrapf(err, "failed to rename database file %s to %s", dbPath, backupPath)
	}

	// Attempt to open the database again. This should be a new database.
	// If this fails, it is a permanent error.
	return Open(dbPath, mode, opts)
}

// fileHasContent checks if we have access to the file with appropriate
// permissions and the file has a non-zero size.
func fileHasContent(dbPath string) bool {
	st, err := os.Stat(dbPath)
	return err == nil && st.Size() > 0
}
