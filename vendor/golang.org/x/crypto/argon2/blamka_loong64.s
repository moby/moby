// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build loong64 && gc && !purego

#include "textflag.h"

// HALF_ROUND applies one BlaMka half-round (mul-add-xor-rotate) to two
// groups of 4 columns, each packed 2-wide into V registers (v0/v2/v4/v6
// hold one pair of columns in lane0/lane1, v1/v3/v5/v7 hold the other).
// t0, t1 are scratch registers, one per independent column group.
#define HALF_ROUND(v0, v1, v2, v3, v4, v5, v6, v7, t0, t1) \
	VMULWEVVWU	v0, v2, t0; \
	VADDV		t0, t0, t0; \
	VADDV		v0, v2, v0; \
	VADDV		v0, t0, v0; \
	VXORV		v6, v0, v6; \
	VROTRV		$32, v6, v6; \
	VMULWEVVWU	v1, v3, t1; \
	VADDV		t1, t1, t1; \
	VADDV		v1, v3, v1; \
	VADDV		v1, t1, v1; \
	VXORV		v7, v1, v7; \
	VROTRV		$32, v7, v7; \
	VMULWEVVWU	v4, v6, t0; \
	VADDV		t0, t0, t0; \
	VADDV		v4, v6, v4; \
	VADDV		v4, t0, v4; \
	VXORV		v2, v4, v2; \
	VROTRV		$24, v2, v2; \
	VMULWEVVWU	v5, v7, t1; \
	VADDV		t1, t1, t1; \
	VADDV		v5, v7, v5; \
	VADDV		v5, t1, v5; \
	VXORV		v3, v5, v3; \
	VROTRV		$24, v3, v3; \
	VMULWEVVWU	v0, v2, t0; \
	VADDV		t0, t0, t0; \
	VADDV		v0, v2, v0; \
	VADDV		v0, t0, v0; \
	VXORV		v6, v0, v6; \
	VROTRV		$16, v6, v6; \
	VMULWEVVWU	v1, v3, t1; \
	VADDV		t1, t1, t1; \
	VADDV		v1, v3, v1; \
	VADDV		v1, t1, v1; \
	VXORV		v7, v1, v7; \
	VROTRV		$16, v7, v7; \
	VMULWEVVWU	v4, v6, t0; \
	VADDV		t0, t0, t0; \
	VADDV		v4, v6, v4; \
	VADDV		v4, t0, v4; \
	VXORV		v2, v4, v2; \
	VROTRV		$63, v2, v2; \
	VMULWEVVWU	v5, v7, t1; \
	VADDV		t1, t1, t1; \
	VADDV		v5, v7, v5; \
	VADDV		v5, t1, v5; \
	VXORV		v3, v5, v3; \
	VROTRV		$63, v3, v3

// SHUFFLE / SHUFFLE_INV: reused verbatim from blake2b_loong64.s -- amd64
// avo source confirms argon2's SHUFFLE/SHUFFLE_INV and blake2b's SHUFFLE
// are byte-for-byte identical macros (diagonalize / undiagonalize),
// operating only on V2..V7 (V0/V1 untouched), with V13 held at zero.
#define SHUFFLE \
	VADDV		V2, V13, V12; \
	VADDV		V6, V13, V16; \
	VADDV		V4, V13, V14; \
	VADDV		V5, V13, V4; \
	VADDV		V14, V13, V5; \
	VSHUF4IV	$9, V3, V2; \
	VSHUF4IV	$9, V12, V3; \
	VSHUF4IV	$3, V7, V6; \
	VSHUF4IV	$3, V16, V7

#define SHUFFLE_INV \
	VADDV		V2, V13, V12; \
	VADDV		V6, V13, V16; \
	VADDV		V4, V13, V14; \
	VADDV		V5, V13, V4; \
	VADDV		V14, V13, V5; \
	VSHUF4IV	$3, V3, V2; \
	VSHUF4IV	$3, V12, V3; \
	VSHUF4IV	$9, V7, V6; \
	VSHUF4IV	$9, V16, V7

// LOAD_MSG_0 / STORE_MSG_0: row pass. `off` is the byte offset of the
// 128-byte (16-uint64) row group; within the macro the 8 loads step by
// 16B (2 uint64 per V register). Called from blamkaLOONG64 with off
// stepping by 128B across the 8 row groups -- do not confuse the two
// strides.
#define LOAD_MSG_0(off) \
	VMOVQ	(off+0)(R4), V0; \
	VMOVQ	(off+16)(R4), V1; \
	VMOVQ	(off+32)(R4), V2; \
	VMOVQ	(off+48)(R4), V3; \
	VMOVQ	(off+64)(R4), V4; \
	VMOVQ	(off+80)(R4), V5; \
	VMOVQ	(off+96)(R4), V6; \
	VMOVQ	(off+112)(R4), V7

#define STORE_MSG_0(off) \
	VMOVQ	V0, (off+0)(R4); \
	VMOVQ	V1, (off+16)(R4); \
	VMOVQ	V2, (off+32)(R4); \
	VMOVQ	V3, (off+48)(R4); \
	VMOVQ	V4, (off+64)(R4); \
	VMOVQ	V5, (off+80)(R4); \
	VMOVQ	V6, (off+96)(R4); \
	VMOVQ	V7, (off+112)(R4)

// LOAD_MSG_1 / STORE_MSG_1: diagonal pass. `off` is the byte offset within
// a 128-byte row group; within the macro the 8 loads step by 128B (one
// element from each of the 8 row groups, mirroring blamka_generic.go's
// t[off], t[128+off], t[256+off], ...). Called from blamkaLOONG64 with
// off stepping by 16B (2 uint64) across the 8 diagonal groups -- the
// inner/outer strides are swapped relative to LOAD_MSG_0/STORE_MSG_0.
#define LOAD_MSG_1(off) \
	VMOVQ	(off+0)(R4), V0; \
	VMOVQ	(off+128)(R4), V1; \
	VMOVQ	(off+256)(R4), V2; \
	VMOVQ	(off+384)(R4), V3; \
	VMOVQ	(off+512)(R4), V4; \
	VMOVQ	(off+640)(R4), V5; \
	VMOVQ	(off+768)(R4), V6; \
	VMOVQ	(off+896)(R4), V7

#define STORE_MSG_1(off) \
	VMOVQ	V0, (off+0)(R4); \
	VMOVQ	V1, (off+128)(R4); \
	VMOVQ	V2, (off+256)(R4); \
	VMOVQ	V3, (off+384)(R4); \
	VMOVQ	V4, (off+512)(R4); \
	VMOVQ	V5, (off+640)(R4); \
	VMOVQ	V6, (off+768)(R4); \
	VMOVQ	V7, (off+896)(R4)

// BLAMKA_ROUND_0(off): one full row pass over the 16-uint64 group at
// byte offset off -- corresponds to a single call to blamkaGeneric
// with the "row" argument order (t[off]..t[off+15]).
#define BLAMKA_ROUND_0(off) \
	LOAD_MSG_0(off); \
	HALF_ROUND(V0, V1, V2, V3, V4, V5, V6, V7, V8, V9); \
	SHUFFLE; \
	HALF_ROUND(V0, V1, V2, V3, V4, V5, V6, V7, V8, V9); \
	SHUFFLE_INV; \
	STORE_MSG_0(off)

// BLAMKA_ROUND_1(off): one full diagonal pass over the 16 uint64s at
// diagonal offset off -- corresponds to a single call to blamkaGeneric
// with the "diagonal" argument order (t[off], t[off+1], t[16+off], ...).
#define BLAMKA_ROUND_1(off) \
	LOAD_MSG_1(off); \
	HALF_ROUND(V0, V1, V2, V3, V4, V5, V6, V7, V8, V9); \
	SHUFFLE; \
	HALF_ROUND(V0, V1, V2, V3, V4, V5, V6, V7, V8, V9); \
	SHUFFLE_INV; \
	STORE_MSG_1(off)

// func blamkaLOONG64(b *block)
TEXT ·blamkaLOONG64(SB), NOSPLIT, $0-8
	MOVV	b+0(FP), R4

	VXORV	V13, V13, V13	// V13 = 0, used by SHUFFLE/SHUFFLE_INV

	// Round 0: 8 row passes, stride 128 bytes (16 uint64s).
	BLAMKA_ROUND_0(0)
	BLAMKA_ROUND_0(128)
	BLAMKA_ROUND_0(256)
	BLAMKA_ROUND_0(384)
	BLAMKA_ROUND_0(512)
	BLAMKA_ROUND_0(640)
	BLAMKA_ROUND_0(768)
	BLAMKA_ROUND_0(896)

	// Round 1: 8 diagonal passes, stride 16 bytes (2 uint64s).
	BLAMKA_ROUND_1(0)
	BLAMKA_ROUND_1(16)
	BLAMKA_ROUND_1(32)
	BLAMKA_ROUND_1(48)
	BLAMKA_ROUND_1(64)
	BLAMKA_ROUND_1(80)
	BLAMKA_ROUND_1(96)
	BLAMKA_ROUND_1(112)

	RET

// func mixBlocksVX(out, a, b, c *block)
TEXT ·mixBlocksVX(SB), NOSPLIT, $0-32
	MOVV	out+0(FP), R4
	MOVV	a+8(FP), R5
	MOVV	b+16(FP), R6
	MOVV	c+24(FP), R7
	MOVV	$1024, R8

	PCALIGN $16
mixloop:
	VMOVQ	(R5), V0
	VMOVQ	(R6), V1
	VMOVQ	(R7), V2
	VXORV	V0, V1, V0
	VXORV	V0, V2, V0
	VMOVQ	V0, (R4)
	ADDV	$16, R4
	ADDV	$16, R5
	ADDV	$16, R6
	ADDV	$16, R7
	SUBV	$16, R8
	BNE	R8, R0, mixloop

	RET

// func xorBlocksVX(out, a, b, c *block)
TEXT ·xorBlocksVX(SB), NOSPLIT, $0-32
	MOVV	out+0(FP), R4
	MOVV	a+8(FP), R5
	MOVV	b+16(FP), R6
	MOVV	c+24(FP), R7
	MOVV	$1024, R8

	PCALIGN $16
xorloop:
	VMOVQ	(R5), V0
	VMOVQ	(R6), V1
	VMOVQ	(R7), V2
	VMOVQ	(R4), V3
	VXORV	V0, V1, V0
	VXORV	V0, V2, V0
	VXORV	V0, V3, V0
	VMOVQ	V0, (R4)
	ADDV	$16, R4
	ADDV	$16, R5
	ADDV	$16, R6
	ADDV	$16, R7
	SUBV	$16, R8
	BNE	R8, R0, xorloop

	RET
