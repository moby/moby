package ieee754

import (
	"encoding/binary"
	"io"
	"math"
)

// DecodeFloat32 decodes a float32 in IEEE 754 binary representation.
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#floating-point%E2%91%A2
func DecodeFloat32(buf []byte) (float32, error) {
	if len(buf) < 4 {
		return 0, io.ErrUnexpectedEOF
	}

	raw := binary.LittleEndian.Uint32(buf[:4])
	return math.Float32frombits(raw), nil
}

// DecodeFloat64 decodes a float64 in IEEE 754 binary representation.
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#floating-point%E2%91%A2
func DecodeFloat64(buf []byte) (float64, error) {
	if len(buf) < 8 {
		return 0, io.ErrUnexpectedEOF
	}

	raw := binary.LittleEndian.Uint64(buf)
	return math.Float64frombits(raw), nil
}
