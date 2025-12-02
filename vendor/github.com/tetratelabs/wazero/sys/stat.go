package sys

import "io/fs"

// Inode is the file serial number, or zero if unknown.
//
// Any constant value will invalidate functions that use this for
// equivalence, such as os.SameFile (Stat_t.Ino).
//
// When zero is returned by a `readdir`, some compilers will attempt to
// get a non-zero value with `lstat`. Those using this for darwin's definition
// of `getdirentries` conflate zero `d_fileno` with a deleted file, so skip the
// entry. See /RATIONALE.md for more on this.
type Inode = uint64

// ^-- Inode is a type alias to consolidate documentation and aid in reference
// searches. While only Stat_t is exposed publicly at the moment, this is used
// internally for Dirent and several function return values.

// EpochNanos is a timestamp in epoch nanoseconds, or zero if unknown.
//
// This defines epoch time the same way as Walltime, except this value is
// packed into an int64. Common conversions are detailed in the examples.
type EpochNanos = int64

// Stat_t is similar to syscall.Stat_t, except available on all operating
// systems, including Windows.
//
// # Notes
//
//   - This is used for WebAssembly ABI emulating the POSIX `stat` system call.
//     See https://pubs.opengroup.org/onlinepubs/9699919799/functions/stat.html
//   - Fields here are required for WebAssembly ABI including wasip1
//     (a.k.a. wasix) and wasi-filesystem (a.k.a. wasip2).
//   - This isn't the same as syscall.Stat_t because wazero supports Windows,
//     which doesn't have that type. runtime.GOOS that has this already also
//     have inconsistent field lengths, which complicates wasm binding.
//   - Use NewStat_t to create this from an existing fs.FileInfo.
//   - For portability, numeric fields are 64-bit when at least one platform
//     defines it that large.
type Stat_t struct {
	// Dev is the device ID of device containing the file.
	Dev uint64

	// Ino is the file serial number, or zero if not available. See Inode for
	// more details including impact returning a zero value.
	Ino Inode

	// Mode is the same as Mode on fs.FileInfo containing bits to identify the
	// type of the file (fs.ModeType) and its permissions (fs.ModePerm).
	Mode fs.FileMode

	// Nlink is the number of hard links to the file.
	//
	// Note: This value is platform-specific and often at least one. Linux will
	// return 1+N for a directory, where BSD (like Darwin) return 2+N, which
	// includes the dot entry.
	Nlink uint64

	// Size is the length in bytes for regular files. For symbolic links, this
	// is length in bytes of the pathname contained in the symbolic link.
	Size int64

	// Atim is the last data access timestamp in epoch nanoseconds.
	Atim EpochNanos

	// Mtim is the last data modification timestamp in epoch nanoseconds.
	Mtim EpochNanos

	// Ctim is the last file status change timestamp in epoch nanoseconds.
	Ctim EpochNanos
}

// NewStat_t fills a new Stat_t from `info`, including any runtime.GOOS-specific
// details from fs.FileInfo `Sys`. When `Sys` is already a *Stat_t, it is
// returned as-is.
//
// # Notes
//
//   - When already in fs.FileInfo `Sys`, Stat_t must be a pointer.
//   - When runtime.GOOS is "windows" Stat_t.Ino will be zero.
//   - When fs.FileInfo `Sys` is nil or unknown, some fields not in fs.FileInfo
//     are defaulted: Stat_t.Atim and Stat_t.Ctim are set to `ModTime`, and
//     are set to ModTime and Stat_t.Nlink is set to 1.
func NewStat_t(info fs.FileInfo) Stat_t {
	// Note: Pointer, not val, for parity with Go, which sets *syscall.Stat_t
	if st, ok := info.Sys().(*Stat_t); ok {
		return *st
	}
	return statFromFileInfo(info)
}

func defaultStatFromFileInfo(info fs.FileInfo) Stat_t {
	st := Stat_t{}
	st.Ino = 0
	st.Dev = 0
	st.Mode = info.Mode()
	st.Nlink = 1
	st.Size = info.Size()
	mtim := info.ModTime().UnixNano() // Set all times to the mod time
	st.Atim = mtim
	st.Mtim = mtim
	st.Ctim = mtim
	return st
}
