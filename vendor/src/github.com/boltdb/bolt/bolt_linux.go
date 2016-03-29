package bolt

import (
	"log"
	"syscall"
	"time"
)

// fdatasync flushes written data to a file descriptor.
func fdatasync(db *DB) error {
	start := time.Now().UnixNano()
	defer func() {
		takenms := (time.Now().UnixNano() - start) / 1e6
		if takenms > 1e3 {
			log.Printf("    fdatasync TOOK %vms", takenms)
		}
	}()
	return syscall.Fdatasync(int(db.file.Fd()))
}
