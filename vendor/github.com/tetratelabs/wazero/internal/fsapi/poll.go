package fsapi

// Pflag are bit flags used for File.Poll. Values, including zero, should not
// be interpreted numerically. Instead, use by constants prefixed with 'POLL'.
//
// # Notes
//
//   - This is like `pollfd.events` flags for `poll` in POSIX. See
//     https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/poll.h.html
type Pflag uint32

// Only define bitflags we support and are needed by `poll_oneoff` in wasip1
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#eventrwflags
const (
	// POLLIN is a read event.
	POLLIN Pflag = 1 << iota

	// POLLOUT is a write event.
	POLLOUT
)
