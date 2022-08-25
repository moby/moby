// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

import (
	"errors"
	"runtime"
)

// DOption is an option for creating a decoder.
type DOption func(*decoderOptions) error

// options retains accumulated state of multiple options.
type decoderOptions struct {
	lowMem         bool
	concurrent     int
	maxDecodedSize uint64
	maxWindowSize  uint64
	dicts          []dict
	ignoreChecksum bool
}

func (o *decoderOptions) setDefault() {
	*o = decoderOptions{
		// use less ram: true for now, but may change.
		lowMem:        true,
		concurrent:    runtime.GOMAXPROCS(0),
		maxWindowSize: MaxWindowSize,
	}
	if o.concurrent > 4 {
		o.concurrent = 4
	}
	o.maxDecodedSize = 64 << 30
}

// WithDecoderLowmem will set whether to use a lower amount of memory,
// but possibly have to allocate more while running.
func WithDecoderLowmem(b bool) DOption {
	return func(o *decoderOptions) error { o.lowMem = b; return nil }
}

// WithDecoderConcurrency sets the number of created decoders.
// When decoding block with DecodeAll, this will limit the number
// of possible concurrently running decodes.
// When decoding streams, this will limit the number of
// inflight blocks.
// When decoding streams and setting maximum to 1,
// no async decoding will be done.
// When a value of 0 is provided GOMAXPROCS will be used.
// By default this will be set to 4 or GOMAXPROCS, whatever is lower.
func WithDecoderConcurrency(n int) DOption {
	return func(o *decoderOptions) error {
		if n < 0 {
			return errors.New("concurrency must be at least 1")
		}
		if n == 0 {
			o.concurrent = runtime.GOMAXPROCS(0)
		} else {
			o.concurrent = n
		}
		return nil
	}
}

// WithDecoderMaxMemory allows to set a maximum decoded size for in-memory
// non-streaming operations or maximum window size for streaming operations.
// This can be used to control memory usage of potentially hostile content.
// Maximum is 1 << 63 bytes. Default is 64GiB.
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
// If several dictionaries with the same ID is provided the last one will be used.
func WithDecoderDicts(dicts ...[]byte) DOption {
	return func(o *decoderOptions) error {
		for _, b := range dicts {
			d, err := loadDict(b)
			if err != nil {
				return err
			}
			o.dicts = append(o.dicts, *d)
		}
		return nil
	}
}

// WithDecoderMaxWindow allows to set a maximum window size for decodes.
// This allows rejecting packets that will cause big memory usage.
// The Decoder will likely allocate more memory based on the WithDecoderLowmem setting.
// If WithDecoderMaxMemory is set to a lower value, that will be used.
// Default is 512MB, Maximum is ~3.75 TB as per zstandard spec.
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

// IgnoreChecksum allows to forcibly ignore checksum checking.
func IgnoreChecksum(b bool) DOption {
	return func(o *decoderOptions) error {
		o.ignoreChecksum = b
		return nil
	}
}
