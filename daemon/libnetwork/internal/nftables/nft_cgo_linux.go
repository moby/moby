//go:build cgo && !static_build && !no_libnftables

package nftables

import (
	"context"
	"errors"
	"fmt"
	"unsafe"

	"github.com/containerd/log"
	"go.opentelemetry.io/otel"
)

// #cgo pkg-config: libnftables
// #cgo nocallback nft_run_cmd_from_buffer
// #cgo nocallback nft_ctx_get_output_buffer
// #cgo nocallback nft_ctx_get_error_buffer
// #include <stdlib.h>
// #include <nftables/libnftables.h>
import "C"

type nftHandle = *C.struct_nft_ctx

// nftApply calls libnftables to execute the nftables commands in nftCmd.
// Acquire t.applyLock before calling this function.
func (t *table) nftApply(ctx context.Context, nftCmd []byte) error {
	ctx, span := otel.Tracer("").Start(ctx, spanPrefix+".nftApply.cgo")
	defer span.End()

	if t.nftHandle == nil {
		handle, err := newNftHandle()
		if err != nil {
			return err
		}
		t.nftHandle = handle
	}

	cCmd := C.CString(string(nftCmd))
	defer C.free(unsafe.Pointer(cCmd))

	ret := C.nft_run_cmd_from_buffer(t.nftHandle, cCmd)
	stdout := C.GoString(C.nft_ctx_get_output_buffer(t.nftHandle))
	stderr := C.GoString(C.nft_ctx_get_error_buffer(t.nftHandle))
	if ret != 0 {
		return fmt.Errorf("libnftables: failed to apply commands (code %d), stderr: %s", int(ret), stderr)
	}
	log.G(ctx).WithFields(log.Fields{"stdout": stdout, "stderr": stderr}).Debug("nftables: updated via libnftables")
	return nil
}

func newNftHandle() (_ *C.struct_nft_ctx, retErr error) {
	handle := C.nft_ctx_new(C.NFT_CTX_DEFAULT)
	if handle == nil {
		return nil, errors.New("libnftables: failed to create new nft handle")
	}
	defer func() {
		if retErr != nil {
			C.nft_ctx_free(handle)
		}
	}()
	if ret := C.nft_ctx_buffer_output(handle); ret != 0 {
		return nil, fmt.Errorf("libnftables: failed to set output buffer (code %d)", int(ret))
	}
	if ret := C.nft_ctx_buffer_error(handle); ret != 0 {
		return nil, fmt.Errorf("libnftables: failed to set error buffer (code %d)", int(ret))
	}
	return handle, nil
}

func (t *table) closeNftHandle() {
	t.applyLock.Lock()
	defer t.applyLock.Unlock()
	if t.nftHandle != nil {
		C.nft_ctx_free(t.nftHandle)
		t.nftHandle = nil
	}
}
