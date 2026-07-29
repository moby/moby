//go:build cgo && !static_build && libnftables

package nftables

import (
	"context"
	"errors"
	"fmt"
	"runtime"
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

func preflight() error {
	return nil
}

// nftCtx owns a libnftables context. The context is freed by [nftCtx.Close],
// or by a runtime cleanup if the nftCtx becomes unreachable without being
// closed.
type nftCtx struct {
	handle  *C.struct_nft_ctx
	cleanup runtime.Cleanup
}

// Apply calls libnftables to execute the nftables commands in nftCmd.
func (h *nftCtx) Apply(ctx context.Context, nftCmd []byte) error {
	if h.handle == nil {
		return errors.New("libnftables: context is closed")
	}

	ctx, span := otel.Tracer("").Start(ctx, spanPrefix+".nftApply.cgo")
	defer span.End()

	cCmd := C.CString(string(nftCmd))
	defer C.free(unsafe.Pointer(cCmd))

	ret := C.nft_run_cmd_from_buffer(h.handle, cCmd)
	stdout := C.GoString(C.nft_ctx_get_output_buffer(h.handle))
	stderr := C.GoString(C.nft_ctx_get_error_buffer(h.handle))
	// Keep h reachable until libnftables is done with its context, so that the
	// cleanup can't free it out from under these calls.
	runtime.KeepAlive(h)
	if ret != 0 {
		return fmt.Errorf("libnftables: failed to apply commands (code %d), stderr: %s", int(ret), stderr)
	}
	log.G(ctx).WithFields(log.Fields{"stdout": stdout, "stderr": stderr}).Debug("nftables: updated via libnftables")
	return nil
}

func newNftCtx() (_ *nftCtx, retErr error) {
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
	h := &nftCtx{handle: handle}
	h.cleanup = runtime.AddCleanup(h, func(handle *C.struct_nft_ctx) {
		C.nft_ctx_free(handle)
	}, handle)
	return h, nil
}

// Close frees the libnftables context. It is idempotent, but h must not be used
// for anything else after it's been closed.
func (h *nftCtx) Close() {
	h.cleanup.Stop()
	if h.handle != nil {
		C.nft_ctx_free(h.handle)
		h.handle = nil
	}
	// Stop only cancels the cleanup if h hasn't already become unreachable, so h
	// must be kept alive across the call to avoid a double free.
	runtime.KeepAlive(h)
}
