//go:build amd64 && !appengine && !noasm && gc

// amd64 stubs and dispatch for the asm loops used by decompress_asm.go.
package huff0

import (
	"github.com/klauspost/compress/internal/cpuinfo"
)

// decompress4x_main_loop_amd64 is an x86 assembler implementation
// of Decompress4X when tablelog > 8.
//
//go:noescape
func decompress4x_main_loop_amd64(ctx *decompress4xContext)

// decompress4x_8b_main_loop_amd64 is an x86 assembler implementation
// of Decompress4X when tablelog <= 8 which decodes 4 entries
// per loop.
//
//go:noescape
func decompress4x_8b_main_loop_amd64(ctx *decompress4xContext)

// decompress1x_main_loop_amd64 is an x86 assembler implementation
// of Decompress1X when tablelog > 8.
//
//go:noescape
func decompress1x_main_loop_amd64(ctx *decompress1xContext)

// decompress1x_main_loop_bmi2 is an x86 with BMI2 assembler implementation
// of Decompress1X when tablelog > 8.
//
//go:noescape
func decompress1x_main_loop_bmi2(ctx *decompress1xContext)

func decompress4x_main_loop_asm(ctx *decompress4xContext) {
	decompress4x_main_loop_amd64(ctx)
}

func decompress4x_8b_main_loop_asm(ctx *decompress4xContext) {
	decompress4x_8b_main_loop_amd64(ctx)
}

func decompress1x_main_loop_asm(ctx *decompress1xContext) {
	if cpuinfo.HasBMI2() {
		decompress1x_main_loop_bmi2(ctx)
	} else {
		decompress1x_main_loop_amd64(ctx)
	}
}
