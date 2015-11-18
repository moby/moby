//+build !noasm
//+build !appengine

// Copyright 2015, Klaus Post, see LICENSE for details.

package flate

import (
	"github.com/klauspost/cpuid"
)

func crc32sse(a []byte) hash
func crc32sseAll(a []byte, dst []hash)
func matchLenSSE4(a, b []byte, max int) int
func histogram(b []byte, h []int32)

func init() {
	useSSE42 = cpuid.CPU.SSE42()
}
