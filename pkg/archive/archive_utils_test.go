package archive

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/containerd/log"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/sys/sequential"
)

// breakoutError is used to differentiate errors related to breaking out
// When testing archive breakout in the unit tests, this error is expected
// in order for the test to pass.
type breakoutError error

type readCloserWrapper struct {
	io.Reader
	closer func() error
	closed atomic.Bool
}

func (r *readCloserWrapper) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		log.G(context.TODO()).Error("subsequent attempt to close readCloserWrapper")
		if log.GetLevel() >= log.DebugLevel {
			log.G(context.TODO()).Errorf("stack trace: %s", string(debug.Stack()))
		}

		return nil
	}
	if r.closer != nil {
		return r.closer()
	}
	return nil
}

// assert that we implement [tar.FileInfoNames].
//
// TODO(thaJeztah): disabled to allow compiling on < go1.23. un-comment once we drop support for older versions of go.
// var _ tar.FileInfoNames = (*nosysFileInfo)(nil)

// nosysFileInfo hides the system-dependent info of the wrapped FileInfo to
// prevent tar.FileInfoHeader from introspecting it and potentially calling into
// glibc.
//
// It implements [tar.FileInfoNames] to further prevent [tar.FileInfoHeader]
// from performing any lookups on go1.23 and up. see https://go.dev/issue/50102
type nosysFileInfo struct {
	os.FileInfo
}

// Uname stubs out looking up username. It implements [tar.FileInfoNames]
// to prevent [tar.FileInfoHeader] from loading libraries to perform
// username lookups.
func (fi nosysFileInfo) Uname() (string, error) {
	return "", nil
}

// Gname stubs out looking up group-name. It implements [tar.FileInfoNames]
// to prevent [tar.FileInfoHeader] from loading libraries to perform
// username lookups.
func (fi nosysFileInfo) Gname() (string, error) {
	return "", nil
}

func (fi nosysFileInfo) Sys() interface{} {
	// A Sys value of type *tar.Header is safe as it is system-independent.
	// The tar.FileInfoHeader function copies the fields into the returned
	// header without performing any OS lookups.
	if sys, ok := fi.FileInfo.Sys().(*tar.Header); ok {
		return sys
	}
	return nil
}

const paxSchilyXattr = "SCHILY.xattr."

// canonicalTarName provides a platform-independent and consistent POSIX-style
// path for files and directories to be archived regardless of the platform.
func canonicalTarName(name string, isDir bool) string {
	name = filepath.ToSlash(name)

	// suffix with '/' for directories
	if isDir && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	return name
}

var copyPool = sync.Pool{
	New: func() interface{} { s := make([]byte, 32*1024); return &s },
}

func copyWithBuffer(dst io.Writer, src io.Reader) error {
	buf := copyPool.Get().(*[]byte)
	_, err := io.CopyBuffer(dst, src, *buf)
	copyPool.Put(buf)
	return err
}

func createTarFile(path, extractDir string, hdr *tar.Header, reader io.Reader, opts *TarOptions) error {
	var (
		Lchown                     = true
		inUserns, bestEffortXattrs bool
		chownOpts                  *idtools.Identity
	)

	// TODO(thaJeztah): make opts a required argument.
	if opts != nil {
		Lchown = !opts.NoLchown
		inUserns = opts.InUserNS // TODO(thaJeztah): consider deprecating opts.InUserNS and detect locally.
		chownOpts = opts.ChownOpts
		bestEffortXattrs = opts.BestEffortXattrs
	}

	// hdr.Mode is in linux format, which we can use for sycalls,
	// but for os.Foo() calls we need the mode converted to os.FileMode,
	// so use hdrInfo.Mode() (they differ for e.g. setuid bits)
	hdrInfo := hdr.FileInfo()

	switch hdr.Typeflag {
	case tar.TypeDir:
		// Create directory unless it exists as a directory already.
		// In that case we just want to merge the two
		if fi, err := os.Lstat(path); !(err == nil && fi.IsDir()) {
			if err := os.Mkdir(path, hdrInfo.Mode()); err != nil {
				return err
			}
		}

	case tar.TypeReg:
		// Source is regular file. We use sequential file access to avoid depleting
		// the standby list on Windows. On Linux, this equates to a regular os.OpenFile.
		file, err := sequential.OpenFile(path, os.O_CREATE|os.O_WRONLY, hdrInfo.Mode())
		if err != nil {
			return err
		}
		if err := copyWithBuffer(file, reader); err != nil {
			_ = file.Close()
			return err
		}
		_ = file.Close()

	case tar.TypeBlock, tar.TypeChar:
		if inUserns { // cannot create devices in a userns
			log.G(context.TODO()).WithFields(log.Fields{"path": path, "type": hdr.Typeflag}).Debug("skipping device nodes in a userns")
			return nil
		}
		// Handle this is an OS-specific way
		if err := handleTarTypeBlockCharFifo(hdr, path); err != nil {
			return err
		}

	case tar.TypeFifo:
		// Handle this is an OS-specific way
		if err := handleTarTypeBlockCharFifo(hdr, path); err != nil {
			if inUserns && errors.Is(err, syscall.EPERM) {
				// In most cases, cannot create a fifo if running in user namespace
				log.G(context.TODO()).WithFields(log.Fields{"error": err, "path": path, "type": hdr.Typeflag}).Debug("creating fifo node in a userns")
				return nil
			}
			return err
		}

	case tar.TypeLink:
		// #nosec G305 -- The target path is checked for path traversal.
		targetPath := filepath.Join(extractDir, hdr.Linkname)
		// check for hardlink breakout
		if !strings.HasPrefix(targetPath, extractDir) {
			return breakoutError(fmt.Errorf("invalid hardlink %q -> %q", targetPath, hdr.Linkname))
		}
		if err := os.Link(targetPath, path); err != nil {
			return err
		}

	case tar.TypeSymlink:
		// 	path 				-> hdr.Linkname = targetPath
		// e.g. /extractDir/path/to/symlink 	-> ../2/file	= /extractDir/path/2/file
		targetPath := filepath.Join(filepath.Dir(path), hdr.Linkname) // #nosec G305 -- The target path is checked for path traversal.

		// the reason we don't need to check symlinks in the path (with FollowSymlinkInScope) is because
		// that symlink would first have to be created, which would be caught earlier, at this very check:
		if !strings.HasPrefix(targetPath, extractDir) {
			return breakoutError(fmt.Errorf("invalid symlink %q -> %q", path, hdr.Linkname))
		}
		if err := os.Symlink(hdr.Linkname, path); err != nil {
			return err
		}

	case tar.TypeXGlobalHeader:
		log.G(context.TODO()).Debug("PAX Global Extended Headers found and ignored")
		return nil

	default:
		return fmt.Errorf("unhandled tar header type %d", hdr.Typeflag)
	}

	// Lchown is not supported on Windows.
	if Lchown && runtime.GOOS != "windows" {
		if chownOpts == nil {
			chownOpts = &idtools.Identity{UID: hdr.Uid, GID: hdr.Gid}
		}
		if err := os.Lchown(path, chownOpts.UID, chownOpts.GID); err != nil {
			var msg string
			if inUserns && errors.Is(err, syscall.EINVAL) {
				msg = " (try increasing the number of subordinate IDs in /etc/subuid and /etc/subgid)"
			}
			return fmt.Errorf("failed to Lchown %q for UID %d, GID %d%s: %w", path, hdr.Uid, hdr.Gid, msg, err)
		}
	}

	var xattrErrs []string
	for key, value := range hdr.PAXRecords {
		xattr, ok := strings.CutPrefix(key, paxSchilyXattr)
		if !ok {
			continue
		}
		if err := lsetxattr(path, xattr, []byte(value), 0); err != nil {
			if bestEffortXattrs && errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EPERM) {
				// EPERM occurs if modifying xattrs is not allowed. This can
				// happen when running in userns with restrictions (ChromeOS).
				xattrErrs = append(xattrErrs, err.Error())
				continue
			}
			return err
		}
	}

	if len(xattrErrs) > 0 {
		log.G(context.TODO()).WithFields(log.Fields{
			"errors": xattrErrs,
		}).Warn("ignored xattrs in archive: underlying filesystem doesn't support them")
	}

	// There is no LChmod, so ignore mode for symlink. Also, this
	// must happen after chown, as that can modify the file mode
	if err := handleLChmod(hdr, path, hdrInfo); err != nil {
		return err
	}

	aTime := boundTime(latestTime(hdr.AccessTime, hdr.ModTime))
	mTime := boundTime(hdr.ModTime)

	// chtimes doesn't support a NOFOLLOW flag atm
	if hdr.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(hdr.Linkname); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			if err := chtimes(path, aTime, mTime); err != nil {
				return err
			}
		}
	} else if hdr.Typeflag != tar.TypeSymlink {
		if err := chtimes(path, aTime, mTime); err != nil {
			return err
		}
	} else {
		if err := lchtimes(path, aTime, mTime); err != nil {
			return err
		}
	}
	return nil
}

// cmdStream executes a command, and returns its stdout as a stream.
// If the command fails to run or doesn't complete successfully, an error
// will be returned, including anything written on stderr.
func cmdStream(cmd *exec.Cmd, input io.Reader) (io.ReadCloser, error) {
	cmd.Stdin = input
	pipeR, pipeW := io.Pipe()
	cmd.Stdout = pipeW
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	// Run the command and return the pipe
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Ensure the command has exited before we clean anything up
	done := make(chan struct{})

	// Copy stdout to the returned pipe
	go func() {
		if err := cmd.Wait(); err != nil {
			pipeW.CloseWithError(fmt.Errorf("%s: %s", err, errBuf.String()))
		} else {
			pipeW.Close()
		}
		close(done)
	}()

	return &readCloserWrapper{
		Reader: pipeR,
		closer: func() error {
			// Close pipeR, and then wait for the command to complete before returning. We have to close pipeR first, as
			// cmd.Wait waits for any non-file stdout/stderr/stdin to close.
			err := pipeR.Close()
			<-done
			return err
		},
	}, nil
}
