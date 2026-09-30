package boltutil

import (
	stderrors "errors"
	"time"

	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/sys/windows"
)

const reopenTimeout = 5 * time.Second

// transactionDB retries a failed reopen when the next transaction arrives.
// The transaction gate keeps the handle stable while a transaction uses it.
func (d *DB) transactionDB() (*bolt.DB, error) {
	if !d.reopenNeeded.Load() {
		return d.bdb, nil
	}
	d.reopenMu.Lock()
	defer d.reopenMu.Unlock()
	if !d.reopenNeeded.Load() {
		return d.bdb, nil
	}
	opts := *d.opts
	if opts.Timeout <= 0 || opts.Timeout > reopenTimeout {
		opts.Timeout = reopenTimeout
	}
	bdb, err := bolt.Open(d.path, d.mode, &opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to reopen database after compaction")
	}
	d.bdb = bdb
	d.reopenNeeded.Store(false)
	return bdb, nil
}

func (d *DB) installCompacted(dst *bolt.DB) (bool, error) {
	// Windows requires both mapped files to be closed before replacement.
	tmp := dst.Path()
	if err := dst.Close(); err != nil {
		return false, err
	}
	var replaced bool
	cause := d.bdb.Close()
	if cause == nil {
		cause = replaceFile(tmp, d.path)
		replaced = cause == nil
	}
	d.reopenNeeded.Store(true)
	_, err := d.transactionDB()
	return replaced, stderrors.Join(cause, err)
}

func replaceFile(from, to string) error {
	src, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	dst, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(src, dst, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
