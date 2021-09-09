package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"time"
)

func adjustTime(itime time.Time) time.Time {
	unixMinTime := time.Unix(0, 0)
	unixMaxTime := maxTime

	// If the modified time is prior to the Unix Epoch, or after the
	// end of Unix Time, os.Chtimes has undefined behavior
	// default to Unix Epoch in this case, just in case

	if itime.Before(unixMinTime) || itime.After(unixMaxTime) {
		return unixMinTime
	}
	return itime
}

// Chtimes changes the access time and modified time of a file at the given path
func Chtimes(name string, atime time.Time, mtime time.Time) error {
	atime = adjustTime(atime)
	mtime = adjustTime(mtime)

	if err := os.Chtimes(name, atime, mtime); err != nil {
		return err
	}

	// Take platform specific action for setting create time.
	return setCTime(name, mtime)
}

// ChtimesNoFollow change the access time and mofified time of a file,
// without following symbol link.
func ChtimesNoFollow(name string, atime time.Time, mtime time.Time) error {
	atime = adjustTime(atime)
	mtime = adjustTime(mtime)

	if err := setAMTimeNoFollow(name, atime, mtime); err != nil {
		return err
	}

	return setCTimeNoFollow(name, mtime)
}
