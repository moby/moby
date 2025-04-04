// Package atomicwriter provides utilities to perform atomic writes to a
// file or set of files.
package atomicwriter

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/moby/sys/sequential"
)

func validateDestination(fileName string) error {
	if fileName == "" {
		return errors.New("file name is empty")
	}
	if dir := filepath.Dir(fileName); dir != "" && dir != "." && dir != ".." {
		di, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("invalid output path: %w", err)
		}
		if !di.IsDir() {
			return fmt.Errorf("invalid output path: %w", &os.PathError{Op: "stat", Path: dir, Err: syscall.ENOTDIR})
		}
	}

	// Deliberately using Lstat here to match the behavior of [os.Rename],
	// which is used when completing the write and does not resolve symlinks.
	fi, err := os.Lstat(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat output path: %w", err)
	}

	switch mode := fi.Mode(); {
	case mode.IsRegular():
		return nil // Regular file
	case mode&os.ModeDir != 0:
		return errors.New("cannot write to a directory")
	case mode&os.ModeSymlink != 0:
		return errors.New("cannot write to a symbolic link directly")
	case mode&os.ModeNamedPipe != 0:
		return errors.New("cannot write to a named pipe (FIFO)")
	case mode&os.ModeSocket != 0:
		return errors.New("cannot write to a socket")
	case mode&os.ModeDevice != 0:
		if mode&os.ModeCharDevice != 0 {
			return errors.New("cannot write to a character device file")
		}
		return errors.New("cannot write to a block device file")
	case mode&os.ModeSetuid != 0:
		return errors.New("cannot write to a setuid file")
	case mode&os.ModeSetgid != 0:
		return errors.New("cannot write to a setgid file")
	case mode&os.ModeSticky != 0:
		return errors.New("cannot write to a sticky bit file")
	default:
		return fmt.Errorf("unknown file mode: %[1]s (%#[1]o)", mode)
	}
}

// New returns a WriteCloser so that writing to it writes to a
// temporary file and closing it atomically changes the temporary file to
// destination path. Writing and closing concurrently is not allowed.
// NOTE: umask is not considered for the file's permissions.
//
// New uses [sequential.CreateTemp] to use sequential file access on Windows,
// avoiding depleting the standby list un-necessarily. On Linux, this equates to
// a regular [os.CreateTemp]. Refer to the [Win32 API documentation] for details
// on sequential file access.
//
// [Win32 API documentation]: https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilea#FILE_FLAG_SEQUENTIAL_SCAN
func New(filename string, perm os.FileMode) (io.WriteCloser, error) {
	if err := validateDestination(filename); err != nil {
		return nil, err
	}
	abspath, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}

	f, err := sequential.CreateTemp(filepath.Dir(abspath), ".tmp-"+filepath.Base(filename))
	if err != nil {
		return nil, err
	}
	return &atomicFileWriter{
		f:    f,
		fn:   abspath,
		perm: perm,
	}, nil
}

// WriteFile atomically writes data to a file named by filename and with the
// specified permission bits. The given filename is created if it does not exist,
// but the destination directory must exist. It can be used as a drop-in replacement
// for [os.WriteFile], but currently does not allow the destination path to be
// a symlink. WriteFile is implemented using [New] for its implementation.
//
// NOTE: umask is not considered for the file's permissions.
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	f, err := New(filename, perm)
	if err != nil {
		return err
	}
	n, err := f.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
		f.(*atomicFileWriter).writeErr = err
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}

type atomicFileWriter struct {
	f        *os.File
	fn       string
	writeErr error
	written  bool
	perm     os.FileMode
}

func (w *atomicFileWriter) Write(dt []byte) (int, error) {
	w.written = true
	n, err := w.f.Write(dt)
	if err != nil {
		w.writeErr = err
	}
	return n, err
}

func (w *atomicFileWriter) Close() (retErr error) {
	defer func() {
		if err := os.Remove(w.f.Name()); !errors.Is(err, os.ErrNotExist) && retErr == nil {
			retErr = err
		}
	}()
	if err := w.f.Sync(); err != nil {
		_ = w.f.Close()
		return err
	}
	if err := w.f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(w.f.Name(), w.perm); err != nil {
		return err
	}
	if w.writeErr == nil && w.written {
		return os.Rename(w.f.Name(), w.fn)
	}
	return nil
}

// WriteSet is used to atomically write a set
// of files and ensure they are visible at the same time.
// Must be committed to a new directory.
type WriteSet struct {
	root string
}

// NewWriteSet creates a new atomic write set to
// atomically create a set of files. The given directory
// is used as the base directory for storing files before
// commit. If no temporary directory is given the system
// default is used.
func NewWriteSet(tmpDir string) (*WriteSet, error) {
	td, err := os.MkdirTemp(tmpDir, "write-set-")
	if err != nil {
		return nil, err
	}

	return &WriteSet{
		root: td,
	}, nil
}

// WriteFile writes a file to the set, guaranteeing the file
// has been synced.
func (ws *WriteSet) WriteFile(filename string, data []byte, perm os.FileMode) error {
	f, err := ws.FileWriter(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	n, err := f.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}

type syncFileCloser struct {
	*os.File
}

func (w syncFileCloser) Close() error {
	err := w.File.Sync()
	if err1 := w.File.Close(); err == nil {
		err = err1
	}
	return err
}

// FileWriter opens a file writer inside the set. The file
// should be synced and closed before calling commit.
//
// FileWriter uses [sequential.OpenFile] to use sequential file access on Windows,
// avoiding depleting the standby list un-necessarily. On Linux, this equates to
// a regular [os.OpenFile]. Refer to the [Win32 API documentation] for details
// on sequential file access.
//
// [Win32 API documentation]: https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilea#FILE_FLAG_SEQUENTIAL_SCAN
func (ws *WriteSet) FileWriter(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
	f, err := sequential.OpenFile(filepath.Join(ws.root, name), flag, perm)
	if err != nil {
		return nil, err
	}
	return syncFileCloser{f}, nil
}

// Cancel cancels the set and removes all temporary data
// created in the set.
func (ws *WriteSet) Cancel() error {
	return os.RemoveAll(ws.root)
}

// Commit moves all created files to the target directory. The
// target directory must not exist and the parent of the target
// directory must exist.
func (ws *WriteSet) Commit(target string) error {
	return os.Rename(ws.root, target)
}

// String returns the location the set is writing to.
func (ws *WriteSet) String() string {
	return ws.root
}
