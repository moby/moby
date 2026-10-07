package linux

import (
	"fmt"
	"io"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/platform"
	"github.com/cilium/ebpf/internal/unix"
)

type auxvPairReader interface {
	Close() error
	ReadAuxvPair() (uint64, uint64, error)
}

// See https://elixir.bootlin.com/linux/v6.5.5/source/include/uapi/linux/auxvec.h
const (
	_AT_NULL         = 0  // End of vector
	_AT_SYSINFO_EHDR = 33 // Offset to vDSO blob in process image
)

type auxvRuntimeReader struct {
	data  [][2]uintptr
	index int
}

func (r *auxvRuntimeReader) Close() error {
	return nil
}

func (r *auxvRuntimeReader) ReadAuxvPair() (uint64, uint64, error) {
	if r.index >= len(r.data)+2 {
		return 0, 0, io.EOF
	}

	// we manually add the (_AT_NULL, _AT_NULL) pair at the end
	// that is not provided by the go runtime
	var tag, value uintptr
	if r.index < len(r.data) {
		tag, value = r.data[r.index][0], r.data[r.index][1]
	} else {
		tag, value = _AT_NULL, _AT_NULL
	}
	r.index += 1
	return uint64(tag), uint64(value), nil
}

func newAuxvRuntimeReader() (auxvPairReader, error) {
	if !platform.IsLinux {
		return nil, fmt.Errorf("read auxv from runtime: %w", internal.ErrNotSupportedOnOS)
	}

	data, err := unix.Auxv()
	if err != nil {
		return nil, fmt.Errorf("read auxv from runtime: %w", err)
	}

	return &auxvRuntimeReader{
		data:  data,
		index: 0,
	}, nil
}
