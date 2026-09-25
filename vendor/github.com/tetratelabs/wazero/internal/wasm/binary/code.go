package binary

import (
	"bytes"
	"fmt"
	"io"
	"math"

	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

func decodeCode(r *bytes.Reader, codeSectionStart uint64, ret *wasm.Code) (err error) {
	ss, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get the size of code: %w", err)
	}
	remaining := int64(ss)

	// Parse #locals.
	ls, bytesRead, err := leb128.DecodeUint32(r)
	remaining -= int64(bytesRead)
	if err != nil {
		return fmt.Errorf("get the size locals: %v", err)
	} else if remaining < 0 {
		return io.EOF
	}

	// Validate the locals.
	bytesRead = 0
	var sum uint64
	for i := uint32(0); i < ls; i++ {
		num, n, err := leb128.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("read n of locals: %v", err)
		} else if remaining < 0 {
			return io.EOF
		}

		sum += uint64(num)

		b, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("read type of local: %v", err)
		}

		bytesRead += n + 1
		switch vt := b; vt {
		case wasm.ValueTypeI32, wasm.ValueTypeF32, wasm.ValueTypeI64, wasm.ValueTypeF64,
			wasm.ValueTypeFuncref, wasm.ValueTypeExternref, wasm.ValueTypeV128:
		default:
			return fmt.Errorf("invalid local type: 0x%x", vt)
		}
	}

	if sum > math.MaxUint32 {
		return fmt.Errorf("too many locals: %d", sum)
	}

	// Rewind the buffer.
	_, err = r.Seek(-int64(bytesRead), io.SeekCurrent)
	if err != nil {
		return err
	}

	localTypes := make([]wasm.ValueType, 0, sum)
	for i := uint32(0); i < ls; i++ {
		num, bytesRead, err := leb128.DecodeUint32(r)
		remaining -= int64(bytesRead) + 1 // +1 for the subsequent ReadByte
		if err != nil {
			return fmt.Errorf("read n of locals: %v", err)
		} else if remaining < 0 {
			return io.EOF
		}

		b, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("read type of local: %v", err)
		}

		for j := uint32(0); j < num; j++ {
			localTypes = append(localTypes, b)
		}
	}

	bodyOffsetInCodeSection := codeSectionStart - uint64(r.Len())
	body := make([]byte, remaining)
	if _, err = io.ReadFull(r, body); err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if endIndex := len(body) - 1; endIndex < 0 || body[endIndex] != wasm.OpcodeEnd {
		return fmt.Errorf("expr not end with OpcodeEnd")
	}

	ret.BodyOffsetInCodeSection = bodyOffsetInCodeSection
	ret.LocalTypes = localTypes
	ret.Body = body
	return nil
}
