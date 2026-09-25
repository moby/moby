package backend

import "github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"

// GoFunctionCallRequiredStackSize returns the size of the stack required for the Go function call.
// argBegin is the index of the first argument in the signature which is not either execution context or module context.
func GoFunctionCallRequiredStackSize(sig *ssa.Signature, argBegin int) (ret, retUnaligned int64) {
	var paramNeededInBytes, resultNeededInBytes int64
	for _, p := range sig.Params[argBegin:] {
		s := int64(p.Size())
		if s < 8 {
			s = 8 // We use uint64 for all basic types, except SIMD v128.
		}
		paramNeededInBytes += s
	}
	for _, r := range sig.Results {
		s := int64(r.Size())
		if s < 8 {
			s = 8 // We use uint64 for all basic types, except SIMD v128.
		}
		resultNeededInBytes += s
	}

	if paramNeededInBytes > resultNeededInBytes {
		ret = paramNeededInBytes
	} else {
		ret = resultNeededInBytes
	}
	retUnaligned = ret
	// Align to 16 bytes.
	ret = (ret + 15) &^ 15
	return
}
