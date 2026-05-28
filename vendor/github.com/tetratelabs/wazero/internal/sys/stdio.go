package sys

import (
	"io"
	"os"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/internal/fsapi"
	"github.com/tetratelabs/wazero/internal/sysfs"
	"github.com/tetratelabs/wazero/sys"
)

// StdinFile is a fs.ModeDevice file for use implementing FdStdin.
// This is safer than reading from os.DevNull as it can never overrun
// operating system file descriptors.
type StdinFile struct {
	noopStdinFile
	io.Reader
}

// Read implements the same method as documented on sys.File
func (f *StdinFile) Read(buf []byte) (int, experimentalsys.Errno) {
	n, err := f.Reader.Read(buf)
	return n, experimentalsys.UnwrapOSError(err)
}

type writerFile struct {
	noopStdoutFile

	w io.Writer
}

// Write implements the same method as documented on sys.File
func (f *writerFile) Write(buf []byte) (int, experimentalsys.Errno) {
	n, err := f.w.Write(buf)
	return n, experimentalsys.UnwrapOSError(err)
}

// noopStdinFile is a fs.ModeDevice file for use implementing FdStdin. This is
// safer than reading from os.DevNull as it can never overrun operating system
// file descriptors.
type noopStdinFile struct {
	noopStdioFile
}

// Read implements the same method as documented on sys.File
func (noopStdinFile) Read([]byte) (int, experimentalsys.Errno) {
	return 0, 0 // Always EOF
}

// Poll implements the same method as documented on fsapi.File
func (noopStdinFile) Poll(flag fsapi.Pflag, timeoutMillis int32) (ready bool, errno experimentalsys.Errno) {
	if flag != fsapi.POLLIN {
		return false, experimentalsys.ENOTSUP
	}
	return true, 0 // always ready to read nothing
}

// noopStdoutFile is a fs.ModeDevice file for use implementing FdStdout and
// FdStderr.
type noopStdoutFile struct {
	noopStdioFile
}

// Write implements the same method as documented on sys.File
func (noopStdoutFile) Write(buf []byte) (int, experimentalsys.Errno) {
	return len(buf), 0 // same as io.Discard
}

type noopStdioFile struct {
	experimentalsys.UnimplementedFile
}

// Stat implements the same method as documented on sys.File
func (noopStdioFile) Stat() (sys.Stat_t, experimentalsys.Errno) {
	return sys.Stat_t{Mode: modeDevice, Nlink: 1}, 0
}

// IsDir implements the same method as documented on sys.File
func (noopStdioFile) IsDir() (bool, experimentalsys.Errno) {
	return false, 0
}

// Close implements the same method as documented on sys.File
func (noopStdioFile) Close() (errno experimentalsys.Errno) { return }

// IsNonblock implements the same method as documented on fsapi.File
func (noopStdioFile) IsNonblock() bool {
	return false
}

// SetNonblock implements the same method as documented on fsapi.File
func (noopStdioFile) SetNonblock(bool) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Poll implements the same method as documented on fsapi.File
func (noopStdioFile) Poll(fsapi.Pflag, int32) (ready bool, errno experimentalsys.Errno) {
	return false, experimentalsys.ENOSYS
}

func stdinFileEntry(r io.Reader) (*FileEntry, error) {
	if r == nil {
		return &FileEntry{Name: "stdin", IsPreopen: true, File: &noopStdinFile{}}, nil
	} else if f, ok := r.(*os.File); ok {
		if f, err := sysfs.NewStdioFile(true, f); err != nil {
			return nil, err
		} else {
			return &FileEntry{Name: "stdin", IsPreopen: true, File: f}, nil
		}
	} else {
		return &FileEntry{Name: "stdin", IsPreopen: true, File: &StdinFile{Reader: r}}, nil
	}
}

func stdioWriterFileEntry(name string, w io.Writer) (*FileEntry, error) {
	if w == nil {
		return &FileEntry{Name: name, IsPreopen: true, File: &noopStdoutFile{}}, nil
	} else if f, ok := w.(*os.File); ok {
		if f, err := sysfs.NewStdioFile(false, f); err != nil {
			return nil, err
		} else {
			return &FileEntry{Name: name, IsPreopen: true, File: f}, nil
		}
	} else {
		return &FileEntry{Name: name, IsPreopen: true, File: &writerFile{w: w}}, nil
	}
}
