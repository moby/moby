package wasip1

import "strconv"

const (
	SockAcceptName   = "sock_accept"
	SockRecvName     = "sock_recv"
	SockSendName     = "sock_send"
	SockShutdownName = "sock_shutdown"
)

// SD Flags indicate which channels on a socket to shut down.
// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-sdflags-flagsu8
const (
	// SD_RD disables further receive operations.
	SD_RD uint8 = 1 << iota //nolint
	// SD_WR disables further send operations.
	SD_WR
)

func SdFlagsString(sdflags int) string {
	return flagsString(sdflagNames[:], sdflags)
}

var sdflagNames = [...]string{
	"RD",
	"WR",
}

// SI Flags are flags provided to sock_send. As there are currently no flags defined, it must be set to zero.
// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-siflags-u16

func SiFlagsString(siflags int) string {
	if siflags == 0 {
		return ""
	}
	return strconv.Itoa(siflags)
}

// RI Flags are flags provided to sock_recv.
// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-riflags-flagsu16
const (
	// RI_RECV_PEEK returns the message without removing it from the socket's receive queue
	RI_RECV_PEEK uint8 = 1 << iota //nolint
	// RI_RECV_WAITALL on byte-stream sockets, block until the full amount of data can be returned.
	RI_RECV_WAITALL
)

func RiFlagsString(riflags int) string {
	return flagsString(riflagNames[:], riflags)
}

var riflagNames = [...]string{
	"RECV_PEEK",
	"RECV_WAITALL",
}

// RO Flags are flags returned by sock_recv.
// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-roflags-flagsu16
const (
	// RO_RECV_DATA_TRUNCATED is returned by sock_recv when message data has been truncated.
	RO_RECV_DATA_TRUNCATED uint8 = 1 << iota //nolint
)

func RoFlagsString(roflags int) string {
	return flagsString(roflagNames[:], roflags)
}

var roflagNames = [...]string{
	"RECV_DATA_TRUNCATED",
}
