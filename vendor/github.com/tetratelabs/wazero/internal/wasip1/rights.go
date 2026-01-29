package wasip1

// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-rights-flagsu64
const (
	// RIGHT_FD_DATASYNC is the right to invoke fd_datasync. If RIGHT_PATH_OPEN
	// is set, includes the right to invoke path_open with FD_DSYNC.
	RIGHT_FD_DATASYNC uint32 = 1 << iota //nolint

	// RIGHT_FD_READ is he right to invoke fd_read and sock_recv. If
	// RIGHT_FD_SYNC is set, includes the right to invoke fd_pread.
	RIGHT_FD_READ

	// RIGHT_FD_SEEK is the right to invoke fd_seek. This flag implies
	// RIGHT_FD_TELL.
	RIGHT_FD_SEEK

	// RIGHT_FDSTAT_SET_FLAGS is the right to invoke fd_fdstat_set_flags.
	RIGHT_FDSTAT_SET_FLAGS

	// RIGHT_FD_SYNC The right to invoke fd_sync. If path_open is set, includes
	// the right to invoke path_open with FD_RSYNC and FD_DSYNC.
	RIGHT_FD_SYNC

	// RIGHT_FD_TELL is the right to invoke fd_seek in such a way that the file
	// offset remains unaltered (i.e., whence::cur with offset zero), or to
	// invoke fd_tell.
	RIGHT_FD_TELL

	// RIGHT_FD_WRITE is the right to invoke fd_write and sock_send. If
	// RIGHT_FD_SEEK is set, includes the right to invoke fd_pwrite.
	RIGHT_FD_WRITE

	// RIGHT_FD_ADVISE is the right to invoke fd_advise.
	RIGHT_FD_ADVISE

	// RIGHT_FD_ALLOCATE is the right to invoke fd_allocate.
	RIGHT_FD_ALLOCATE

	// RIGHT_PATH_CREATE_DIRECTORY is the right to invoke
	// path_create_directory.
	RIGHT_PATH_CREATE_DIRECTORY

	// RIGHT_PATH_CREATE_FILE when RIGHT_PATH_OPEN is set, the right to invoke
	// path_open with O_CREAT.
	RIGHT_PATH_CREATE_FILE

	// RIGHT_PATH_LINK_SOURCE is the right to invoke path_link with the file
	// descriptor as the source directory.
	RIGHT_PATH_LINK_SOURCE

	// RIGHT_PATH_LINK_TARGET is the right to invoke path_link with the file
	// descriptor as the target directory.
	RIGHT_PATH_LINK_TARGET

	// RIGHT_PATH_OPEN is the right to invoke path_open.
	RIGHT_PATH_OPEN

	// RIGHT_FD_READDIR is the right to invoke fd_readdir.
	RIGHT_FD_READDIR

	// RIGHT_PATH_READLINK is the right to invoke path_readlink.
	RIGHT_PATH_READLINK

	// RIGHT_PATH_RENAME_SOURCE is the right to invoke path_rename with the
	// file descriptor as the source directory.
	RIGHT_PATH_RENAME_SOURCE

	// RIGHT_PATH_RENAME_TARGET is the right to invoke path_rename with the
	// file descriptor as the target directory.
	RIGHT_PATH_RENAME_TARGET

	// RIGHT_PATH_FILESTAT_GET is the right to invoke path_filestat_get.
	RIGHT_PATH_FILESTAT_GET

	// RIGHT_PATH_FILESTAT_SET_SIZE is the right to change a file's size (there
	// is no path_filestat_set_size). If RIGHT_PATH_OPEN is set, includes the
	// right to invoke path_open with O_TRUNC.
	RIGHT_PATH_FILESTAT_SET_SIZE

	// RIGHT_PATH_FILESTAT_SET_TIMES is the right to invoke
	// path_filestat_set_times.
	RIGHT_PATH_FILESTAT_SET_TIMES

	// RIGHT_FD_FILESTAT_GET is the right to invoke fd_filestat_get.
	RIGHT_FD_FILESTAT_GET

	// RIGHT_FD_FILESTAT_SET_SIZE is the right to invoke fd_filestat_set_size.
	RIGHT_FD_FILESTAT_SET_SIZE

	// RIGHT_FD_FILESTAT_SET_TIMES is the right to invoke
	// fd_filestat_set_times.
	RIGHT_FD_FILESTAT_SET_TIMES

	// RIGHT_PATH_SYMLINK is the right to invoke path_symlink.
	RIGHT_PATH_SYMLINK

	// RIGHT_PATH_REMOVE_DIRECTORY is the right to invoke
	// path_remove_directory.
	RIGHT_PATH_REMOVE_DIRECTORY

	// RIGHT_PATH_UNLINK_FILE is the right to invoke path_unlink_file.
	RIGHT_PATH_UNLINK_FILE

	// RIGHT_POLL_FD_READWRITE when RIGHT_FD_READ is set, includes the right to
	// invoke poll_oneoff to subscribe to eventtype::fd_read. If RIGHT_FD_WRITE
	// is set, includes the right to invoke poll_oneoff to subscribe to
	// eventtype::fd_write.
	RIGHT_POLL_FD_READWRITE

	// RIGHT_SOCK_SHUTDOWN is the right to invoke sock_shutdown.
	RIGHT_SOCK_SHUTDOWN
)

func RightsString(rights int) string {
	return flagsString(rightNames[:], rights)
}

var rightNames = [...]string{
	"FD_DATASYNC",
	"FD_READ",
	"FD_SEEK",
	"FDSTAT_SET_FLAGS",
	"FD_SYNC",
	"FD_TELL",
	"FD_WRITE",
	"FD_ADVISE",
	"FD_ALLOCATE",
	"PATH_CREATE_DIRECTORY",
	"PATH_CREATE_FILE",
	"PATH_LINK_SOURCE",
	"PATH_LINK_TARGET",
	"PATH_OPEN",
	"FD_READDIR",
	"PATH_READLINK",
	"PATH_RENAME_SOURCE",
	"PATH_RENAME_TARGET",
	"PATH_FILESTAT_GET",
	"PATH_FILESTAT_SET_SIZE",
	"PATH_FILESTAT_SET_TIMES",
	"FD_FILESTAT_GET",
	"FD_FILESTAT_SET_SIZE",
	"FD_FILESTAT_SET_TIMES",
	"PATH_SYMLINK",
	"PATH_REMOVE_DIRECTORY",
	"PATH_UNLINK_FILE",
	"POLL_FD_READWRITE",
	"SOCK_SHUTDOWN",
}
