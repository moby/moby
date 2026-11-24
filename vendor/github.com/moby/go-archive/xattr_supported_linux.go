package archive

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/moby/go-archive/internal/archiveoptions"
	"github.com/moby/go-archive/internal/unshare"
	"golang.org/x/sys/unix"
)

var noattr = unix.ENODATA

// withXattrPath pins the parent through root and preserves the final
// component for lsetxattr's no-follow semantics, including dangling symlinks.
func withXattrPath(root *os.Root, name string, opts *archiveoptions.Options, apply func(string) error) error {
	parent, err := root.OpenFile(filepath.Dir(name), unix.O_PATH|unix.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer parent.Close()

	base := filepath.Base(name)
	if opts == nil || opts.ProcSelfFD == nil {
		procPath := "/proc/self/fd/" + strconv.FormatUint(uint64(parent.Fd()), 10) + "/" + base
		err := apply(procPath)
		if err == nil || !errors.Is(err, os.ErrNotExist) {
			return err
		}
		// A missing leaf must keep its error. Fall back only when procfs
		// itself is absent in this thread's filesystem context.
		if _, procErr := os.Stat("/proc/self/fd"); !errors.Is(procErr, os.ErrNotExist) {
			return err
		}
	}

	// Extraction can run without procfs, including callers that did not
	// prepare options through chrootarchive. Change cwd only on an isolated
	// thread so lsetxattr still addresses the pinned parent directly.
	return unshare.WithCwd(parent, func() error { return apply(base) })
}
