package wasi_snapshot_preview1

import (
	"context"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental/sys"
	socketapi "github.com/tetratelabs/wazero/internal/sock"
	"github.com/tetratelabs/wazero/internal/sysfs"
	"github.com/tetratelabs/wazero/internal/wasip1"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// sockAccept is the WASI function named SockAcceptName which accepts a new
// incoming connection.
//
// See: https://github.com/WebAssembly/WASI/blob/0ba0c5e2e37625ca5a6d3e4255a998dfaa3efc52/phases/snapshot/docs.md#sock_accept
// and https://github.com/WebAssembly/WASI/pull/458
var sockAccept = newHostFunc(
	wasip1.SockAcceptName,
	sockAcceptFn,
	[]wasm.ValueType{i32, i32, i32},
	"fd", "flags", "result.fd",
)

func sockAcceptFn(_ context.Context, mod api.Module, params []uint64) (errno sys.Errno) {
	mem := mod.Memory()
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	flags := uint32(params[1])
	resultFd := uint32(params[2])
	nonblock := flags&uint32(wasip1.FD_NONBLOCK) != 0

	var connFD int32
	if connFD, errno = fsc.SockAccept(fd, nonblock); errno == 0 {
		mem.WriteUint32Le(resultFd, uint32(connFD))
	}
	return
}

// sockRecv is the WASI function named SockRecvName which receives a
// message from a socket.
//
// See: https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-sock_recvfd-fd-ri_data-iovec_array-ri_flags-riflags---errno-size-roflags
var sockRecv = newHostFunc(
	wasip1.SockRecvName,
	sockRecvFn,
	[]wasm.ValueType{i32, i32, i32, i32, i32, i32},
	"fd", "ri_data", "ri_data_len", "ri_flags", "result.ro_datalen", "result.ro_flags",
)

func sockRecvFn(_ context.Context, mod api.Module, params []uint64) sys.Errno {
	mem := mod.Memory()
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	riData := uint32(params[1])
	riDataCount := uint32(params[2])
	riFlags := uint8(params[3])
	resultRoDatalen := uint32(params[4])
	resultRoFlags := uint32(params[5])

	var conn socketapi.TCPConn
	if e, ok := fsc.LookupFile(fd); !ok {
		return sys.EBADF // Not open
	} else if conn, ok = e.File.(socketapi.TCPConn); !ok {
		return sys.EBADF // Not a conn
	}

	if riFlags & ^(wasip1.RI_RECV_PEEK|wasip1.RI_RECV_WAITALL) != 0 {
		return sys.ENOTSUP
	}

	if riFlags&wasip1.RI_RECV_PEEK != 0 {
		// Each record in riData is of the form:
		// type iovec struct { buf *uint8; bufLen uint32 }
		// This means that the first `uint32` is a `buf *uint8`.
		firstIovecBufAddr, ok := mem.ReadUint32Le(riData)
		if !ok {
			return sys.EINVAL
		}
		// Read bufLen
		firstIovecBufLen, ok := mem.ReadUint32Le(riData + 4)
		if !ok {
			return sys.EINVAL
		}
		firstIovecBuf, ok := mem.Read(firstIovecBufAddr, firstIovecBufLen)
		if !ok {
			return sys.EINVAL
		}
		n, err := conn.Recvfrom(firstIovecBuf, sysfs.MSG_PEEK)
		if err != 0 {
			return err
		}
		mem.WriteUint32Le(resultRoDatalen, uint32(n))
		mem.WriteUint16Le(resultRoFlags, 0)
		return 0
	}

	// If riFlags&wasip1.RECV_WAITALL != 0 then we should
	// do a blocking operation until all data has been retrieved;
	// otherwise we are able to return earlier.
	// For simplicity, we currently wait all regardless the flag.
	bufSize, errno := readv(mem, riData, riDataCount, conn.Read)
	if errno != 0 {
		return errno
	}
	mem.WriteUint32Le(resultRoDatalen, bufSize)
	mem.WriteUint16Le(resultRoFlags, 0)
	return 0
}

// sockSend is the WASI function named SockSendName which sends a message
// on a socket.
//
// See: https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-sock_sendfd-fd-si_data-ciovec_array-si_flags-siflags---errno-size
var sockSend = newHostFunc(
	wasip1.SockSendName,
	sockSendFn,
	[]wasm.ValueType{i32, i32, i32, i32, i32},
	"fd", "si_data", "si_data_len", "si_flags", "result.so_datalen",
)

func sockSendFn(_ context.Context, mod api.Module, params []uint64) sys.Errno {
	mem := mod.Memory()
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	siData := uint32(params[1])
	siDataCount := uint32(params[2])
	siFlags := uint32(params[3])
	resultSoDatalen := uint32(params[4])

	if siFlags != 0 {
		return sys.ENOTSUP
	}

	var conn socketapi.TCPConn
	if e, ok := fsc.LookupFile(fd); !ok {
		return sys.EBADF // Not open
	} else if conn, ok = e.File.(socketapi.TCPConn); !ok {
		return sys.EBADF // Not a conn
	}

	bufSize, errno := writev(mem, siData, siDataCount, conn.Write)
	if errno != 0 {
		return errno
	}
	mem.WriteUint32Le(resultSoDatalen, bufSize)
	return 0
}

// sockShutdown is the WASI function named SockShutdownName which shuts
// down socket send and receive channels.
//
// See: https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-sock_shutdownfd-fd-how-sdflags---errno
var sockShutdown = newHostFunc(wasip1.SockShutdownName, sockShutdownFn, []wasm.ValueType{i32, i32}, "fd", "how")

func sockShutdownFn(_ context.Context, mod api.Module, params []uint64) sys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	how := uint8(params[1])

	var conn socketapi.TCPConn
	if e, ok := fsc.LookupFile(fd); !ok {
		return sys.EBADF // Not open
	} else if conn, ok = e.File.(socketapi.TCPConn); !ok {
		return sys.EBADF // Not a conn
	}

	sysHow := 0

	switch how {
	case wasip1.SD_RD | wasip1.SD_WR:
		sysHow = socketapi.SHUT_RD | socketapi.SHUT_WR
	case wasip1.SD_RD:
		sysHow = socketapi.SHUT_RD
	case wasip1.SD_WR:
		sysHow = socketapi.SHUT_WR
	default:
		return sys.EINVAL
	}

	// TODO: Map this instead of relying on syscall symbols.
	return conn.Shutdown(sysHow)
}
