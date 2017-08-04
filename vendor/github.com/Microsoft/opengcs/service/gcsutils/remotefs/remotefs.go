// +build !windows

package remotefs

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/symlink"
	"golang.org/x/sys/unix"
)

// Func is the function definition for a generic remote fs function
// The input to the function is any serialized structs / data from in and the string slice
// from args. The output of the function will be serialized and written to out.
type Func func(stdin io.Reader, stdout io.Writer, args []string) error

// Commands provide a string -> remotefs function mapping.
// This is useful for commandline programs that will receive a string
// as the function to execute.
var Commands = map[string]Func{
	StatCmd:           Stat,
	LstatCmd:          Lstat,
	ReadlinkCmd:       Readlink,
	MkdirCmd:          Mkdir,
	MkdirAllCmd:       MkdirAll,
	RemoveCmd:         Remove,
	RemoveAllCmd:      RemoveAll,
	LinkCmd:           Link,
	SymlinkCmd:        Symlink,
	LchmodCmd:         Lchmod,
	LchownCmd:         Lchown,
	MknodCmd:          Mknod,
	MkfifoCmd:         Mkfifo,
	OpenFileCmd:       OpenFile,
	ReadFileCmd:       ReadFile,
	WriteFileCmd:      WriteFile,
	ReadDirCmd:        ReadDir,
	ResolvePathCmd:    ResolvePath,
	ExtractArchiveCmd: ExtractArchive,
	ArchivePathCmd:    ArchivePath,
}

// Stat functions like os.Stat.
// Args:
// - args[0] is the path
// Out:
// - out = FileInfo object
func Stat(in io.Reader, out io.Writer, args []string) error {
	return stat(in, out, args, os.Stat)
}

// Lstat functions like os.Lstat.
// Args:
// - args[0] is the path
// Out:
// - out = FileInfo object
func Lstat(in io.Reader, out io.Writer, args []string) error {
	return stat(in, out, args, os.Lstat)
}

func stat(in io.Reader, out io.Writer, args []string, statfunc func(string) (os.FileInfo, error)) error {
	if len(args) < 1 {
		return ErrInvalid
	}

	fi, err := statfunc(args[0])
	if err != nil {
		return err
	}

	info := FileInfo{
		NameVar:    fi.Name(),
		SizeVar:    fi.Size(),
		ModeVar:    fi.Mode(),
		ModTimeVar: fi.ModTime().UnixNano(),
		IsDirVar:   fi.IsDir(),
	}

	buf, err := json.Marshal(info)
	if err != nil {
		return err
	}

	if _, err := out.Write(buf); err != nil {
		return err
	}
	return nil
}

// Readlink works like os.Readlink
// In:
//  - args[0] is path
// Out:
//  - Write link result to out
func Readlink(in io.Reader, out io.Writer, args []string) error {
	if len(args) < 1 {
		return ErrInvalid
	}

	l, err := os.Readlink(args[0])
	if err != nil {
		return err
	}

	if _, err := out.Write([]byte(l)); err != nil {
		return err
	}
	return nil
}

// Mkdir works like os.Mkdir
// Args:
// - args[0] is the path
// - args[1] is the permissions in octal (like 0755)
func Mkdir(in io.Reader, out io.Writer, args []string) error {
	return mkdir(in, out, args, os.Mkdir)
}

// MkdirAll works like os.MkdirAll.
// Args:
// - args[0] is the path
// - args[1] is the permissions in octal (like 0755)
func MkdirAll(in io.Reader, out io.Writer, args []string) error {
	return mkdir(in, out, args, os.MkdirAll)
}

func mkdir(in io.Reader, out io.Writer, args []string, mkdirFunc func(string, os.FileMode) error) error {
	if len(args) < 2 {
		return ErrInvalid
	}

	perm, err := strconv.ParseUint(args[1], 8, 32)
	if err != nil {
		return err
	}
	return mkdirFunc(args[0], os.FileMode(perm))
}

// Remove works like os.Remove
// Args:
//	- args[0] is the path
func Remove(in io.Reader, out io.Writer, args []string) error {
	return remove(in, out, args, os.Remove)
}

// RemoveAll works like os.RemoveAll
// Args:
//  - args[0] is the path
func RemoveAll(in io.Reader, out io.Writer, args []string) error {
	return remove(in, out, args, os.RemoveAll)
}

func remove(in io.Reader, out io.Writer, args []string, removefunc func(string) error) error {
	if len(args) < 1 {
		return ErrInvalid
	}
	return removefunc(args[0])
}

// Link works like os.Link
// Args:
//  - args[0] = old path name (link source)
//  - args[1] = new path name (link dest)
func Link(in io.Reader, out io.Writer, args []string) error {
	return link(in, out, args, os.Link)
}

// Symlink works like os.Symlink
// Args:
//  - args[0] = old path name (link source)
//  - args[1] = new path name (link dest)
func Symlink(in io.Reader, out io.Writer, args []string) error {
	return link(in, out, args, os.Symlink)
}

func link(in io.Reader, out io.Writer, args []string, linkfunc func(string, string) error) error {
	if len(args) < 2 {
		return ErrInvalid
	}
	return linkfunc(args[0], args[1])
}

// Lchmod changes permission of the given file without following symlinks
// Args:
//  - args[0] = path
//  - args[1] = permission mode in octal (like 0755)
func Lchmod(in io.Reader, out io.Writer, args []string) error {
	if len(args) < 2 {
		return ErrInvalid
	}

	perm, err := strconv.ParseUint(args[1], 8, 32)
	if err != nil {
		return err
	}

	path := args[0]
	if !filepath.IsAbs(path) {
		path, err = filepath.Abs(path)
		if err != nil {
			return err
		}
	}
	return unix.Fchmodat(0, path, uint32(perm), unix.AT_SYMLINK_NOFOLLOW)
}

// Lchown works like os.Lchown
// Args:
//  - args[0] = path
//  - args[1] = uid in base 10
//  - args[2] = gid in base 10
func Lchown(in io.Reader, out io.Writer, args []string) error {
	if len(args) < 3 {
		return ErrInvalid
	}

	uid, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return err
	}

	gid, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return err
	}
	return os.Lchown(args[0], int(uid), int(gid))
}

// Mknod works like syscall.Mknod
// Args:
//  - args[0] = path
//  - args[1] = permission mode in octal (like 0755)
//  - args[2] = major device number in base 10
//  - args[3] = minor device number in base 10
func Mknod(in io.Reader, out io.Writer, args []string) error {
	if len(args) < 4 {
		return ErrInvalid
	}

	perm, err := strconv.ParseUint(args[1], 8, 32)
	if err != nil {
		return err
	}

	major, err := strconv.ParseInt(args[2], 10, 32)
	if err != nil {
		return err
	}

	minor, err := strconv.ParseInt(args[3], 10, 32)
	if err != nil {
		return err
	}

	dev := unix.Mkdev(uint32(major), uint32(minor))
	return unix.Mknod(args[0], uint32(perm), int(dev))
}

// Mkfifo creates a FIFO special file with the given path name and permissions
// Args:
// 	- args[0] = path
//  - args[1] = permission mode in octal (like 0755)
func Mkfifo(in io.Reader, out io.Writer, args []string) error {
	if len(args) < 2 {
		return ErrInvalid
	}

	perm, err := strconv.ParseUint(args[1], 8, 32)
	if err != nil {
		return err
	}
	return unix.Mkfifo(args[0], uint32(perm))
}

// OpenFile works like os.OpenFile. To manage the file pointer state,
// this function acts as a single file "file server" with Read/Write/Close
// being serialized control codes from in.
// Args:
//  - args[0] = path
//  - args[1] = flag in base 10
//  - args[2] = permission mode in octal (like 0755)
func OpenFile(in io.Reader, out io.Writer, args []string) (err error) {
	defer func() {
		if err != nil {
			// error code will be serialized by the caller, so don't write it here
			WriteFileHeader(out, &FileHeader{Cmd: CmdFailed}, nil)
		}
	}()

	if len(args) < 3 {
		return ErrInvalid
	}

	flag, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		return err
	}

	perm, err := strconv.ParseUint(args[2], 8, 32)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(args[0], int(flag), os.FileMode(perm))
	if err != nil {
		return err
	}

	// Signal the client that OpenFile succeeded
	if err := WriteFileHeader(out, &FileHeader{Cmd: CmdOK}, nil); err != nil {
		return err
	}

	for {
		hdr, err := ReadFileHeader(in)
		if err != nil {
			return err
		}

		var buf []byte
		switch hdr.Cmd {
		case Read:
			buf = make([]byte, hdr.Size, hdr.Size)
			n, err := f.Read(buf)
			if err != nil {
				return err
			}
			buf = buf[:n]
		case Write:
			if _, err := io.CopyN(f, in, int64(hdr.Size)); err != nil {
				return err
			}
		case Seek:
			seekHdr := &SeekHeader{}
			if err := binary.Read(in, binary.BigEndian, seekHdr); err != nil {
				return err
			}
			res, err := f.Seek(seekHdr.Offset, int(seekHdr.Whence))
			if err != nil {
				return err
			}
			buffer := &bytes.Buffer{}
			if err := binary.Write(buffer, binary.BigEndian, res); err != nil {
				return err
			}
			buf = buffer.Bytes()
		case Close:
			if err := f.Close(); err != nil {
				return err
			}
		default:
			return ErrUnknown
		}

		retHdr := &FileHeader{
			Cmd:  CmdOK,
			Size: uint64(len(buf)),
		}
		if err := WriteFileHeader(out, retHdr, buf); err != nil {
			return err
		}

		if hdr.Cmd == Close {
			break
		}
	}
	return nil
}

// ReadFile works like ioutil.ReadFile but instead writes the file to a writer
// Args:
//  - args[0] = path
// Out:
//  - Write file contents to out
func ReadFile(in io.Reader, out io.Writer, args []string) error {
	if len(args) < 1 {
		return ErrInvalid
	}

	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(out, f); err != nil {
		return nil
	}
	return nil
}

// WriteFile works like ioutil.WriteFile but instead reads the file from a reader
// Args:
//  - args[0] = path
//  - args[1] = permission mode in octal (like 0755)
//  - input data stream from in
func WriteFile(in io.Reader, out io.Writer, args []string) error {
	if len(args) < 2 {
		return ErrInvalid
	}

	perm, err := strconv.ParseUint(args[1], 8, 32)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(args[0], os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(perm))
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, in); err != nil {
		return err
	}
	return nil
}

// ReadDir works like *os.File.Readdir but instead writes the result to a writer
// Args:
//  - args[0] = path
//  - args[1] = number of directory entries to return. If <= 0, return all entries in directory
func ReadDir(in io.Reader, out io.Writer, args []string) error {
	if len(args) < 2 {
		return ErrInvalid
	}

	n, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		return err
	}

	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()

	infos, err := f.Readdir(int(n))
	if err != nil {
		return err
	}

	fileInfos := make([]FileInfo, len(infos))
	for i := range infos {
		fileInfos[i] = FileInfo{
			NameVar:    infos[i].Name(),
			SizeVar:    infos[i].Size(),
			ModeVar:    infos[i].Mode(),
			ModTimeVar: infos[i].ModTime().UnixNano(),
			IsDirVar:   infos[i].IsDir(),
		}
	}

	buf, err := json.Marshal(fileInfos)
	if err != nil {
		return err
	}

	if _, err := out.Write(buf); err != nil {
		return err
	}
	return nil
}

// ResolvePath works like docker's symlink.FollowSymlinkInScope.
// It takens in a `path` and a `root` and evaluates symlinks in `path`
// as if they were scoped in `root`. `path` must be a child path of `root`.
// In other words, `path` must have `root` as a prefix.
// Example:
// path=/foo/bar -> /baz
// root=/foo,
// Expected result = /foo/baz
//
// Args:
// - args[0] is `path`
// - args[1] is `root`
// Out:
// - Write resolved path to stdout
func ResolvePath(in io.Reader, out io.Writer, args []string) error {
	if len(args) < 2 {
		return ErrInvalid
	}
	res, err := symlink.FollowSymlinkInScope(args[0], args[1])
	if err != nil {
		return err
	}
	if _, err = out.Write([]byte(res)); err != nil {
		return err
	}
	return nil
}

// ExtractArchive extracts the archive read from in.
// Args:
// - in = size of json | json of archive.TarOptions | input tar stream
// - args[0] = extract directory name
func ExtractArchive(in io.Reader, out io.Writer, args []string) error {
	if len(args) < 1 {
		return ErrInvalid
	}

	opts, err := ReadTarOptions(in)
	if err != nil {
		return err
	}

	if err := archive.Untar(in, args[0], opts); err != nil {
		return err
	}
	return nil
}

// ArchivePath archives the given directory and writes it to out.
// Args:
// - in = size of json | json of archive.TarOptions
// - args[0] = source directory name
// Out:
// - out = tar file of the archive
func ArchivePath(in io.Reader, out io.Writer, args []string) error {
	if len(args) < 1 {
		return ErrInvalid
	}

	opts, err := ReadTarOptions(in)
	if err != nil {
		return err
	}

	r, err := archive.TarWithOptions(args[0], opts)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, r); err != nil {
		return err
	}
	return nil
}
