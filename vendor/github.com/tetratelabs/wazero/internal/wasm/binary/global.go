package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// decodeGlobal returns the api.Global decoded with the WebAssembly 1.0 (20191205) Binary Format.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-global
func decodeGlobal(r *bytes.Reader, enabledFeatures api.CoreFeatures, ret *wasm.Global) (err error) {
	ret.Type, err = decodeGlobalType(r)
	if err != nil {
		return err
	}

	err = decodeConstantExpression(r, enabledFeatures, &ret.Init)
	return
}

// decodeGlobalType returns the wasm.GlobalType decoded with the WebAssembly 1.0 (20191205) Binary Format.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-globaltype
func decodeGlobalType(r *bytes.Reader) (wasm.GlobalType, error) {
	vt, err := decodeValueTypes(r, 1)
	if err != nil {
		return wasm.GlobalType{}, fmt.Errorf("read value type: %w", err)
	}

	ret := wasm.GlobalType{
		ValType: vt[0],
	}

	b, err := r.ReadByte()
	if err != nil {
		return wasm.GlobalType{}, fmt.Errorf("read mutablity: %w", err)
	}

	switch mut := b; mut {
	case 0x00: // not mutable
	case 0x01: // mutable
		ret.Mutable = true
	default:
		return wasm.GlobalType{}, fmt.Errorf("%w for mutability: %#x != 0x00 or 0x01", ErrInvalidByte, mut)
	}
	return ret, nil
}
