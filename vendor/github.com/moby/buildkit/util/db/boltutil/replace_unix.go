//go:build !windows

package boltutil

import (
	"errors"
	"os"
	"path/filepath"

	bolt "go.etcd.io/bbolt"
)

func (d *DB) installCompacted(dst *bolt.DB) (bool, error) {
	// Keep both handles locked through replacement; the new database is
	// already usable even if syncing the directory fails.
	if err := os.Rename(dst.Path(), d.path); err != nil { // #nosec G703 -- Both files belong to the daemon's database directory.
		return false, err
	}
	old := d.bdb
	d.bdb = dst
	return true, errors.Join(syncDir(filepath.Dir(d.path)), old.Close())
}

func syncDir(dir string) error {
	f, err := os.Open(dir) // #nosec G703 -- This is the daemon's database directory.
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
