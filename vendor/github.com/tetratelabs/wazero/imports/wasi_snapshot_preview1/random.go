package wasi_snapshot_preview1

import (
	"context"
	"io"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/internal/wasip1"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// randomGet is the WASI function named RandomGetName which writes random
// data to a buffer.
//
// # Parameters
//
//   - buf: api.Memory offset to write random values
//   - bufLen: size of random data in bytes
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - sys.EFAULT: `buf` or `bufLen` point to an offset out of memory
//   - sys.EIO: a file system error
//
// For example, if underlying random source was seeded like
// `rand.NewSource(42)`, we expect api.Memory to contain:
//
//	                   bufLen (5)
//	          +--------------------------+
//	          |                        	 |
//	[]byte{?, 0x53, 0x8c, 0x7f, 0x96, 0xb1, ?}
//	    buf --^
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-random_getbuf-pointeru8-bufLen-size---errno
var randomGet = newHostFunc(wasip1.RandomGetName, randomGetFn, []api.ValueType{i32, i32}, "buf", "buf_len")

func randomGetFn(_ context.Context, mod api.Module, params []uint64) sys.Errno {
	sysCtx := mod.(*wasm.ModuleInstance).Sys
	randSource := sysCtx.RandSource()
	buf, bufLen := uint32(params[0]), uint32(params[1])

	randomBytes, ok := mod.Memory().Read(buf, bufLen)
	if !ok { // out-of-range
		return sys.EFAULT
	}

	// We can ignore the returned n as it only != byteCount on error
	if _, err := io.ReadAtLeast(randSource, randomBytes, int(bufLen)); err != nil {
		return sys.EIO
	}

	return 0
}
