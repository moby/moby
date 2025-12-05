package wasip1

import (
	"fmt"
)

const (
	FdAdviseName           = "fd_advise"
	FdAllocateName         = "fd_allocate"
	FdCloseName            = "fd_close"
	FdDatasyncName         = "fd_datasync"
	FdFdstatGetName        = "fd_fdstat_get"
	FdFdstatSetFlagsName   = "fd_fdstat_set_flags"
	FdFdstatSetRightsName  = "fd_fdstat_set_rights"
	FdFilestatGetName      = "fd_filestat_get"
	FdFilestatSetSizeName  = "fd_filestat_set_size"
	FdFilestatSetTimesName = "fd_filestat_set_times"
	FdPreadName            = "fd_pread"
	FdPrestatGetName       = "fd_prestat_get"
	FdPrestatDirNameName   = "fd_prestat_dir_name"
	FdPwriteName           = "fd_pwrite"
	FdReadName             = "fd_read"
	FdReaddirName          = "fd_readdir"
	FdRenumberName         = "fd_renumber"
	FdSeekName             = "fd_seek"
	FdSyncName             = "fd_sync"
	FdTellName             = "fd_tell"
	FdWriteName            = "fd_write"

	PathCreateDirectoryName  = "path_create_directory"
	PathFilestatGetName      = "path_filestat_get"
	PathFilestatSetTimesName = "path_filestat_set_times"
	PathLinkName             = "path_link"
	PathOpenName             = "path_open"
	PathReadlinkName         = "path_readlink"
	PathRemoveDirectoryName  = "path_remove_directory"
	PathRenameName           = "path_rename"
	PathSymlinkName          = "path_symlink"
	PathUnlinkFileName       = "path_unlink_file"
)

// oflags are open flags used by path_open
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-oflags-flagsu16
const (
	// O_CREAT creates a file if it does not exist.
	O_CREAT uint16 = 1 << iota //nolint
	// O_DIRECTORY fails if not a directory.
	O_DIRECTORY
	// O_EXCL fails if file already exists.
	O_EXCL //nolint
	// O_TRUNC truncates the file to size 0.
	O_TRUNC //nolint
)

func OflagsString(oflags int) string {
	return flagsString(oflagNames[:], oflags)
}

var oflagNames = [...]string{
	"CREAT",
	"DIRECTORY",
	"EXCL",
	"TRUNC",
}

// file descriptor flags
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fdflags
const (
	FD_APPEND uint16 = 1 << iota //nolint
	FD_DSYNC
	FD_NONBLOCK
	FD_RSYNC
	FD_SYNC
)

func FdFlagsString(fdflags int) string {
	return flagsString(fdflagNames[:], fdflags)
}

var fdflagNames = [...]string{
	"APPEND",
	"DSYNC",
	"NONBLOCK",
	"RSYNC",
	"SYNC",
}

// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#lookupflags
const (
	// LOOKUP_SYMLINK_FOLLOW expands a path if it resolves into a symbolic
	// link.
	LOOKUP_SYMLINK_FOLLOW uint16 = 1 << iota //nolint
)

var lookupflagNames = [...]string{
	"SYMLINK_FOLLOW",
}

func LookupflagsString(lookupflags int) string {
	return flagsString(lookupflagNames[:], lookupflags)
}

// DirentSize is the size of the dirent struct, which should be followed by the
// length of a file name.
const DirentSize = uint32(24)

const (
	FILETYPE_UNKNOWN uint8 = iota
	FILETYPE_BLOCK_DEVICE
	FILETYPE_CHARACTER_DEVICE
	FILETYPE_DIRECTORY
	FILETYPE_REGULAR_FILE
	FILETYPE_SOCKET_DGRAM
	FILETYPE_SOCKET_STREAM
	FILETYPE_SYMBOLIC_LINK
)

// FiletypeName returns string name of the file type.
func FiletypeName(filetype uint8) string {
	if int(filetype) < len(filetypeToString) {
		return filetypeToString[filetype]
	}
	return fmt.Sprintf("filetype(%d)", filetype)
}

var filetypeToString = [...]string{
	"UNKNOWN",
	"BLOCK_DEVICE",
	"CHARACTER_DEVICE",
	"DIRECTORY",
	"REGULAR_FILE",
	"SOCKET_DGRAM",
	"SOCKET_STREAM",
	"SYMBOLIC_LINK",
}

// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fstflags
const (
	FstflagsAtim uint16 = 1 << iota
	FstflagsAtimNow
	FstflagsMtim
	FstflagsMtimNow
)

var fstflagNames = [...]string{
	"ATIM",
	"ATIM_NOW",
	"MTIM",
	"MTIM_NOW",
}

func FstflagsString(fdflags int) string {
	return flagsString(fstflagNames[:], fdflags)
}

// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-advice-enumu8
const (
	FdAdviceNormal byte = iota
	FdAdviceSequential
	FdAdviceRandom
	FdAdviceWillNeed
	FdAdviceDontNeed
	FdAdviceNoReuse
)
