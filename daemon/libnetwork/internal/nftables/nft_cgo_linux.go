//go:build cgo && !static_build && libnftables

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

func preflight() error {
	return nil
}

type nftCtx C.struct_nft_ctx

// Apply calls libnftables to execute the nftables commands in nftCmd.
func (h *nftCtx) Apply(ctx context.Context, nftCmd []byte) error {
	ctx, span := otel.Tracer("").Start(ctx, spanPrefix+".nftApply.cgo")
	defer span.End()

	cCmd := C.CString(string(nftCmd))
	defer C.free(unsafe.Pointer(cCmd))

	ret := C.nft_run_cmd_from_buffer((*C.struct_nft_ctx)(h), cCmd)
	stdout := C.GoString(C.nft_ctx_get_output_buffer((*C.struct_nft_ctx)(h)))
	stderr := C.GoString(C.nft_ctx_get_error_buffer((*C.struct_nft_ctx)(h)))
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
	return (*nftCtx)(handle), nil
}

func (h *nftCtx) Close() {
	C.nft_ctx_free((*C.struct_nft_ctx)(h))
}
