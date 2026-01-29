package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

func decodeImport(
	r *bytes.Reader,
	idx uint32,
	memorySizer memorySizer,
	memoryLimitPages uint32,
	enabledFeatures api.CoreFeatures,
	ret *wasm.Import,
) (err error) {
	if ret.Module, _, err = decodeUTF8(r, "import module"); err != nil {
		err = fmt.Errorf("import[%d] error decoding module: %w", idx, err)
		return
	}

	if ret.Name, _, err = decodeUTF8(r, "import name"); err != nil {
		err = fmt.Errorf("import[%d] error decoding name: %w", idx, err)
		return
	}

	b, err := r.ReadByte()
	if err != nil {
		err = fmt.Errorf("import[%d] error decoding type: %w", idx, err)
		return
	}
	ret.Type = b
	switch ret.Type {
	case wasm.ExternTypeFunc:
		ret.DescFunc, _, err = leb128.DecodeUint32(r)
	case wasm.ExternTypeTable:
		err = decodeTable(r, enabledFeatures, &ret.DescTable)
	case wasm.ExternTypeMemory:
		ret.DescMem, err = decodeMemory(r, enabledFeatures, memorySizer, memoryLimitPages)
	case wasm.ExternTypeGlobal:
		ret.DescGlobal, err = decodeGlobalType(r)
	default:
		err = fmt.Errorf("%w: invalid byte for importdesc: %#x", ErrInvalidByte, b)
	}
	if err != nil {
		err = fmt.Errorf("import[%d] %s[%s.%s]: %w", idx, wasm.ExternTypeName(ret.Type), ret.Module, ret.Name, err)
	}
	return
}
