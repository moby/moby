package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// decodeMemory returns the api.Memory decoded with the WebAssembly 1.0 (20191205) Binary Format.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-memory
func decodeMemory(
	r *bytes.Reader,
	enabledFeatures api.CoreFeatures,
	memorySizer func(minPages uint32, maxPages *uint32) (min, capacity, max uint32),
	memoryLimitPages uint32,
) (*wasm.Memory, error) {
	min, maxP, shared, err := decodeLimitsType(r)
	if err != nil {
		return nil, err
	}

	if shared {
		if !enabledFeatures.IsEnabled(experimental.CoreFeaturesThreads) {
			return nil, fmt.Errorf("shared memory requested but threads feature not enabled")
		}

		// This restriction may be lifted in the future.
		// https://webassembly.github.io/threads/core/binary/types.html#memory-types
		if maxP == nil {
			return nil, fmt.Errorf("shared memory requires a maximum size to be specified")
		}
	}

	min, capacity, max := memorySizer(min, maxP)
	mem := &wasm.Memory{Min: min, Cap: capacity, Max: max, IsMaxEncoded: maxP != nil, IsShared: shared}

	return mem, mem.Validate(memoryLimitPages)
}
