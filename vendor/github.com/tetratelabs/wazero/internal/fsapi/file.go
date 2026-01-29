package fsapi

import experimentalsys "github.com/tetratelabs/wazero/experimental/sys"

// File includes methods not yet ready to document for end users, notably
// non-blocking functionality.
//
// Particularly, Poll is subject to debate. For example, whether a user should
// be able to choose how to implement timeout or not. Currently, this interface
// allows the user to choose to sleep or use native polling, and which choice
// they make impacts thread behavior as summarized here:
// https://github.com/tetratelabs/wazero/pull/1606#issuecomment-1665475516
type File interface {
	experimentalsys.File

	// IsNonblock returns true if the file was opened with O_NONBLOCK, or
	// SetNonblock was successfully enabled on this file.
	//
	// # Notes
	//
	//   - This might not match the underlying state of the file descriptor if
	//     the file was not opened via OpenFile.
	IsNonblock() bool

	// SetNonblock toggles the non-blocking mode (O_NONBLOCK) of this file.
	//
	// # Errors
	//
	// A zero Errno is success. The below are expected otherwise:
	//   - ENOSYS: the implementation does not support this function.
	//   - EBADF: the file or directory was closed.
	//
	// # Notes
	//
	//   - This is like syscall.SetNonblock and `fcntl` with O_NONBLOCK in
	//     POSIX. See https://pubs.opengroup.org/onlinepubs/9699919799/functions/fcntl.html
	SetNonblock(enable bool) experimentalsys.Errno

	// Poll returns if the file has data ready to be read or written.
	//
	// # Parameters
	//
	// The `flag` parameter determines which event to await, such as POLLIN,
	// POLLOUT, or a combination like `POLLIN|POLLOUT`.
	//
	// The `timeoutMillis` parameter is how long to block for an event, or
	// interrupted, in milliseconds. There are two special values:
	//   - zero returns immediately
	//   - any negative value blocks any amount of time
	//
	// # Results
	//
	// `ready` means there was data ready to read or written. False can mean no
	// event was ready or `errno` is not zero.
	//
	// A zero `errno` is success. The below are expected otherwise:
	//   - ENOSYS: the implementation does not support this function.
	//   - ENOTSUP: the implementation does not the flag combination.
	//   - EINTR: the call was interrupted prior to an event.
	//
	// # Notes
	//
	//   - This is like `poll` in POSIX, for a single file.
	//     See https://pubs.opengroup.org/onlinepubs/9699919799/functions/poll.html
	//   - No-op files, such as those which read from /dev/null, should return
	//     immediately true, as data will never become available.
	//   - See /RATIONALE.md for detailed notes including impact of blocking.
	Poll(flag Pflag, timeoutMillis int32) (ready bool, errno experimentalsys.Errno)
}
