package sys

// Oflag are flags used for FS.OpenFile. Values, including zero, should not be
// interpreted numerically. Instead, use by constants prefixed with 'O_' with
// special casing noted below.
//
// # Notes
//
//   - O_RDONLY, O_RDWR and O_WRONLY are mutually exclusive, while the other
//     flags can coexist bitwise.
//   - This is like `flag` in os.OpenFile and `oflag` in POSIX. See
//     https://pubs.opengroup.org/onlinepubs/9699919799/functions/open.html
type Oflag uint32

// This is a subset of oflags to reduce implementation burden. `wasip1` splits
// these across `oflags` and `fdflags`. We can't rely on the Go `os` package,
// as it is missing some values. Any flags added will be defined in POSIX
// order, as needed by functions that explicitly document accepting them.
//
// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-oflags-flagsu16
// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fdflags-flagsu16
const (
	// O_RDONLY is like os.O_RDONLY
	O_RDONLY Oflag = iota

	// O_RDWR is like os.O_RDWR
	O_RDWR

	// O_WRONLY is like os.O_WRONLY
	O_WRONLY

	// Define bitflags as they are in POSIX `open`: alphabetically

	// O_APPEND is like os.O_APPEND
	O_APPEND Oflag = 1 << iota

	// O_CREAT is link os.O_CREATE
	O_CREAT

	// O_DIRECTORY is defined on some platforms as syscall.O_DIRECTORY.
	//
	// Note: This ensures that the opened file is a directory. Those emulating
	// on platforms that don't support the O_DIRECTORY, can double-check the
	// result with File.IsDir (or stat) and err if not a directory.
	O_DIRECTORY

	// O_DSYNC is defined on some platforms as syscall.O_DSYNC.
	O_DSYNC

	// O_EXCL is defined on some platforms as syscall.O_EXCL.
	O_EXCL

	// O_NOFOLLOW is defined on some platforms as syscall.O_NOFOLLOW.
	//
	// Note: This allows programs to ensure that if the opened file is a
	// symbolic link, the link itself is opened instead of its target.
	O_NOFOLLOW

	// O_NONBLOCK is defined on some platforms as syscall.O_NONBLOCK.
	O_NONBLOCK

	// O_RSYNC is defined on some platforms as syscall.O_RSYNC.
	O_RSYNC

	// O_SYNC is defined on some platforms as syscall.O_SYNC.
	O_SYNC

	// O_TRUNC is defined on some platforms as syscall.O_TRUNC.
	O_TRUNC
)
