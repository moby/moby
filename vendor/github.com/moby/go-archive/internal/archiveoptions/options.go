// Package archiveoptions defines internal options shared between archive and
// chrootarchive.
package archiveoptions

import "os"

// Options contains extraction resources supplied by internal callers.
type Options struct {
	// ProcSelfFD references /proc/self/fd as opened before entering a chroot.
	// The caller retains ownership of the file.
	ProcSelfFD *os.File
}
