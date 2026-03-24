// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

import (
	"errors"
	"fmt"
	"math/bits"
	"runtime"
)

// DOption is an option for creating a decoder.
type DOption func(*decoderOptions) error

// options retains accumulated state of multiple options.
type decoderOptions struct {
	lowMem          bool
	concurrent      int
	maxDecodedSize  uint64
	maxWindowSize   uint64
	dicts           map[uint32]*dict
	ignoreChecksum  bool
	limitToCap      bool
	decodeBufsBelow int
	resetOpt        bool
}

func (o *decoderOptions) setDefault() {
	*o = decoderOptions{
		// use less ram: true for now, but may change.
		lowMem:          true,
		concurrent:      runtime.GOMAXPROCS(0),
		maxWindowSize:   MaxWindowSize,
		decodeBufsBelow: 128 << 10,
	}
	if o.concurrent > 4 {
		o.concurrent = 4
	}
	o.maxDecodedSize = 64 << 30
}

// WithDecoderLowmem will set whether to use a lower amount of memory,
// but possibly have to allocate more while running.
// Cannot be changed with ResetWithOptions.
func WithDecoderLowmem(b bool) DOption {
	return func(o *decoderOptions) error {
		if o.resetOpt && b != o.lowMem {
			return errors.New("WithDecoderLowmem cannot be changed on Reset")
		}
		o.lowMem = b
		return nil
	}
}

// WithDecoderConcurrency sets the number of created decoders.
// When decoding block with DecodeAll, this will limit the number
// of possible concurrently running decodes.
// When decoding streams, this will limit the number of
// inflight blocks.
// When decoding streams and setting maximum to 1,
// no async decoding will be done.
// The value supplied must be at least 0.
// When a value of 0 is provided GOMAXPROCS will be used.
// By default this will be set to 4 or GOMAXPROCS, whatever is lower.
// Cannot be changed with ResetWithOptions.
func WithDecoderConcurrency(n int) DOption {
	return func(o *decoderOptions) error {
		if n < 0 {
			return errors.New("concurrency must be at least 0")
		}
		newVal := n
		if n == 0 {
			newVal = runtime.GOMAXPROCS(0)
		}
		if o.resetOpt && newVal != o.concurrent {
			return errors.New("WithDecoderConcurrency cannot be changed on Reset")
		}
		o.concurrent = newVal
		return nil
	}
}

// WithDecoderMaxMemory allows to set a maximum decoded size for in-memory
// non-streaming operations or maximum window size for streaming operations.
// This can be used to control memory usage of potentially hostile content.
// Maximum is 1 << 63 bytes. Default is 64GiB.
// Can be changed with ResetWithOptions.
func WithDecoderMaxMemory(n uint64) DOption {
	return func(o *decoderOptions) error {
		if n == 0 {
			return errors.New("WithDecoderMaxMemory must be at least 1")
		}
		if n > 1<<63 {
			return errors.New("WithDecoderMaxmemory must be less than 1 << 63")
		}
		o.maxDecodedSize = n
		return nil
	}
}

// WithDecoderDicts allows to register one or more dictionaries for the decoder.
//
// Each slice in dict must be in the [dictionary format] produced by
// "zstd --train" from the Zstandard reference implementation.
//
// If several dictionaries with the same ID are provided, the last one will be used.
// Can be changed with ResetWithOptions.
//
// [dictionary format]: https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md#dictionary-format
func WithDecoderDicts(dicts ...[]byte) DOption {
	return func(o *decoderOptions) error {
		if o.dicts == nil {
			o.dicts = make(map[uint32]*dict)
		}
		for _, b := range dicts {
			d, err := loadDict(b)
			if err != nil {
				return err
			}
			o.dicts[d.id] = d
		}
		return nil
	}
}

// WithDecoderDictRaw registers a dictionary that may be used by the decoder.
// The slice content can be arbitrary data.
// Can be changed with ResetWithOptions.
func WithDecoderDictRaw(id uint32, content []byte) DOption {
	return func(o *decoderOptions) error {
		if bits.UintSize > 32 && uint(len(content)) > dictMaxLength {
			return fmt.Errorf("dictionary of size %d > 2GiB too large", len(content))
		}
		if o.dicts == nil {
			o.dicts = make(map[uint32]*dict)
		}
		o.dicts[id] = &dict{id: id, content: content, offsets: [3]int{1, 4, 8}}
		return nil
	}
}

// WithDecoderMaxWindow allows to set a maximum window size for decodes.
// This allows rejecting packets that will cause big memory usage.
// The Decoder will likely allocate more memory based on the WithDecoderLowmem setting.
// If WithDecoderMaxMemory is set to a lower value, that will be used.
// Default is 512MB, Maximum is ~3.75 TB as per zstandard spec.
// Can be changed with ResetWithOptions.
func WithDecoderMaxWindow(size uint64) DOption {
	return func(o *decoderOptions) error {
		if size < MinWindowSize {
			return errors.New("WithMaxWindowSize must be at least 1KB, 1024 bytes")
		}
		if size > (1<<41)+7*(1<<38) {
			return errors.New("WithMaxWindowSize must be less than (1<<41) + 7*(1<<38) ~ 3.75TB")
		}
		o.maxWindowSize = size
		return nil
	}
}

// WithDecodeAllCapLimit will limit DecodeAll to decoding cap(dst)-len(dst) bytes,
// or any size set in WithDecoderMaxMemory.
// This can be used to limit decoding to a specific maximum output size.
// Disabled by default.
// Can be changed with ResetWithOptions.
func WithDecodeAllCapLimit(b bool) DOption {
	return func(o *decoderOptions) error {
		o.limitToCap = b
		return nil
	}
}

// WithDecodeBuffersBelow will fully decode readers that have a
// `Bytes() []byte` and `Len() int` interface similar to bytes.Buffer.
// This typically uses less allocations but will have the full decompressed object in memory.
// Note that DecodeAllCapLimit will disable this, as well as giving a size of 0 or less.
// Default is 128KiB.
// Cannot be changed with ResetWithOptions.
func WithDecodeBuffersBelow(size int) DOption {
	return func(o *decoderOptions) error {
		if o.resetOpt && size != o.decodeBufsBelow {
			return errors.New("WithDecodeBuffersBelow cannot be changed on Reset")
		}
		o.decodeBufsBelow = size
		return nil
	}
}

// IgnoreChecksum allows to forcibly ignore checksum checking.
// Can be changed with ResetWithOptions.
func IgnoreChecksum(b bool) DOption {
	return func(o *decoderOptions) error {
		o.ignoreChecksum = b
		return nil
	}
}

// WithDecoderDictDelete removes dictionaries by ID.
// If no ids are passed, all dictionaries are deleted.
// Should be used with ResetWithOptions.
func WithDecoderDictDelete(ids ...uint32) DOption {
	return func(o *decoderOptions) error {
		if len(ids) == 0 {
			clear(o.dicts)
		}
		for _, id := range ids {
			delete(o.dicts, id)
		}
		return nil
	}
}
