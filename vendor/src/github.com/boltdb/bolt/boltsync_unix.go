// +build !windows,!plan9,!linux,!openbsd

package bolt

import (
	"log"
	"time"
)

// fdatasync flushes written data to a file descriptor.
func fdatasync(db *DB) error {
	start := time.Now().UnixNano()
	defer func() {
		takenms := (time.Now().UnixNano() - start) / 1e6
		if takenms > thr_ms {
			log.Printf("    fdatasync TOOK %vms", takenms)
		}
	}()
	return db.file.Sync()
}
