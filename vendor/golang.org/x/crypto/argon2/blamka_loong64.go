// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build loong64 && gc && !purego

package argon2

import "golang.org/x/sys/cpu"

func init() {
	// useLSX (declared in blamka_generic.go) gates the LSX vector path.
	// If the running CPU lacks LSX support, processBlockVX falls back
	// to processBlockGeneric.
	useLSX = cpu.Loong64.HasLSX
}

//go:noescape
func blamkaLOONG64(b *block)

//go:noescape
func mixBlocksVX(out, a, b, c *block)

//go:noescape
func xorBlocksVX(out, a, b, c *block)

func processBlock(out, in1, in2 *block) {
	processBlockVX(out, in1, in2, false)
}

func processBlockXOR(out, in1, in2 *block) {
	processBlockVX(out, in1, in2, true)
}

func processBlockVX(out, in1, in2 *block, xor bool) {
	if !useLSX {
		processBlockGeneric(out, in1, in2, xor)
		return
	}

	var t block
	mixBlocksVX(&t, in1, in2, &t)
	blamkaLOONG64(&t)
	if xor {
		xorBlocksVX(out, in1, in2, &t)
	} else {
		mixBlocksVX(out, in1, in2, &t)
	}
}
