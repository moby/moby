package binary

import (
	"bytes"

	"github.com/tetratelabs/wazero/internal/wasm"
)

// decodeCustomSection deserializes the data **not** associated with the "name" key in SectionIDCustom.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#custom-section%E2%91%A0
func decodeCustomSection(r *bytes.Reader, name string, limit uint64) (result *wasm.CustomSection, err error) {
	buf := make([]byte, limit)
	_, err = r.Read(buf)

	result = &wasm.CustomSection{
		Name: name,
		Data: buf,
	}

	return
}
