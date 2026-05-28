package binary

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// dataSegmentPrefix represents three types of data segments.
//
// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/binary/modules.html#data-section
type dataSegmentPrefix = uint32

const (
	// dataSegmentPrefixActive is the prefix for the version 1.0 compatible data segment, which is classified as "active" in 2.0.
	dataSegmentPrefixActive dataSegmentPrefix = 0x0
	// dataSegmentPrefixPassive prefixes the "passive" data segment as in version 2.0 specification.
	dataSegmentPrefixPassive dataSegmentPrefix = 0x1
	// dataSegmentPrefixActiveWithMemoryIndex is the active prefix with memory index encoded which is defined for futur use as of 2.0.
	dataSegmentPrefixActiveWithMemoryIndex dataSegmentPrefix = 0x2
)

func decodeDataSegment(r *bytes.Reader, enabledFeatures api.CoreFeatures, ret *wasm.DataSegment) (err error) {
	dataSegmentPrefx, _, err := leb128.DecodeUint32(r)
	if err != nil {
		err = fmt.Errorf("read data segment prefix: %w", err)
		return
	}

	if dataSegmentPrefx != dataSegmentPrefixActive {
		if err = enabledFeatures.RequireEnabled(api.CoreFeatureBulkMemoryOperations); err != nil {
			err = fmt.Errorf("non-zero prefix for data segment is invalid as %w", err)
			return
		}
	}

	switch dataSegmentPrefx {
	case dataSegmentPrefixActive,
		dataSegmentPrefixActiveWithMemoryIndex:
		// Active data segment as in
		// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/binary/modules.html#data-section
		if dataSegmentPrefx == 0x2 {
			d, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return fmt.Errorf("read memory index: %v", err)
			} else if d != 0 {
				return fmt.Errorf("memory index must be zero but was %d", d)
			}
		}

		err = decodeConstantExpression(r, enabledFeatures, &ret.OffsetExpression)
		if err != nil {
			return fmt.Errorf("read offset expression: %v", err)
		}
	case dataSegmentPrefixPassive:
		// Passive data segment doesn't need const expr nor memory index encoded.
		// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/binary/modules.html#data-section
		ret.Passive = true
	default:
		err = fmt.Errorf("invalid data segment prefix: 0x%x", dataSegmentPrefx)
		return
	}

	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		err = fmt.Errorf("get the size of vector: %v", err)
		return
	}

	ret.Init = make([]byte, vs)
	if _, err = io.ReadFull(r, ret.Init); err != nil {
		err = fmt.Errorf("read bytes for init: %v", err)
	}
	return
}
