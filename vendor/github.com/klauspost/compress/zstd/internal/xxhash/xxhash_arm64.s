// +build gc,!purego,!noasm

#include "textflag.h"

// Register allocation.
#define digest	R1
#define h	R2 // Return value.
#define p	R3 // Input pointer.
#define len	R4
#define nblocks	R5 // len / 32.
#define prime1	R7
#define prime2	R8
#define prime3	R9
#define prime4	R10
#define prime5	R11
#define v1	R12
#define v2	R13
#define v3	R14
#define v4	R15
#define x1	R20
#define x2	R21
#define x3	R22
#define x4	R23

#define round(acc, x) \
	MADD prime2, acc, x, acc \
	ROR  $64-31, acc         \
	MUL  prime1, acc         \

// x = round(0, x).
#define round0(x) \
	MUL prime2, x \
	ROR $64-31, x \
	MUL prime1, x \

#define mergeRound(x) \
	round0(x)                 \
	EOR  x, h                 \
	MADD h, prime4, prime1, h \

// Update v[1-4] with 32-byte blocks. Assumes len >= 32.
#define blocksLoop() \
	LSR     $5, len, nblocks \
	PCALIGN $16              \
	loop:                    \
	LDP.P   32(p), (x1, x2)  \
	round(v1, x1)            \
	LDP     -16(p), (x3, x4) \
	round(v2, x2)            \
	SUB     $1, nblocks      \
	round(v3, x3)            \
	round(v4, x4)            \
	CBNZ    nblocks, loop    \

// The primes are repeated here to ensure that they're stored
// in a contiguous array, so we can load them with LDP.
DATA primes<> +0(SB)/8, $11400714785074694791
DATA primes<> +8(SB)/8, $14029467366897019727
DATA primes<>+16(SB)/8, $1609587929392839161
DATA primes<>+24(SB)/8, $9650029242287828579
DATA primes<>+32(SB)/8, $2870177450012600261
GLOBL primes<>(SB), NOPTR+RODATA, $40

// func Sum64(b []byte) uint64
TEXT ·Sum64(SB), NOFRAME+NOSPLIT, $0-32
	LDP b_base+0(FP), (p, len)

	LDP  primes<> +0(SB), (prime1, prime2)
	LDP  primes<>+16(SB), (prime3, prime4)
	MOVD primes<>+32(SB), prime5

	CMP  $32, len
	CSEL LO, prime5, ZR, h // if len < 32 { h = prime5 } else { h = 0 }
	BLO  afterLoop

	ADD  prime1, prime2, v1
	MOVD prime2, v2
	MOVD $0, v3
	NEG  prime1, v4

	blocksLoop()

	ROR $64-1, v1, x1
	ROR $64-7, v2, x2
	ADD x1, x2
	ROR $64-12, v3, x3
	ROR $64-18, v4, x4
	ADD x3, x4
	ADD x2, x4, h

	mergeRound(v1)
	mergeRound(v2)
	mergeRound(v3)
	mergeRound(v4)

afterLoop:
	ADD len, h

	TBZ   $4, len, try8
	LDP.P 16(p), (x1, x2)

	round0(x1)
	ROR  $64-27, h
	EOR  x1 @> 64-27, h, h
	MADD h, prime4, prime1, h

	round0(x2)
	ROR  $64-27, h
	EOR  x2 @> 64-27, h
	MADD h, prime4, prime1, h

try8:
	TBZ    $3, len, try4
	MOVD.P 8(p), x1

	round0(x1)
	ROR  $64-27, h
	EOR  x1 @> 64-27, h
	MADD h, prime4, prime1, h

try4:
	TBZ     $2, len, try2
	MOVWU.P 4(p), x2

	MUL  prime1, x2
	ROR  $64-23, h
	EOR  x2 @> 64-23, h
	MADD h, prime3, prime2, h

try2:
	TBZ     $1, len, try1
	MOVHU.P 2(p), x3
	AND     $255, x3, x1
	LSR     $8, x3, x2

	MUL prime5, x1
	ROR $64-11, h
	EOR x1 @> 64-11, h
	MUL prime1, h

	MUL prime5, x2
	ROR $64-11, h
	EOR x2 @> 64-11, h
	MUL prime1, h

try1:
	TBZ   $0, len, end
	MOVBU (p), x4

	MUL prime5, x4
	ROR $64-11, h
	EOR x4 @> 64-11, h
	MUL prime1, h

end:
	EOR h >> 33, h
	MUL prime2, h
	EOR h >> 29, h
	MUL prime3, h
	EOR h >> 32, h

	MOVD h, ret+24(FP)
	RET

// func writeBlocks(d *Digest, b []byte) int
//
// Assumes len(b) >= 32.
TEXT ·writeBlocks(SB), NOFRAME+NOSPLIT, $0-40
	LDP primes<>(SB), (prime1, prime2)

	// Load state. Assume v[1-4] are stored contiguously.
	MOVD d+0(FP), digest
	LDP  0(digest), (v1, v2)
	LDP  16(digest), (v3, v4)

	LDP b_base+8(FP), (p, len)

	blocksLoop()

	// Store updated state.
	STP (v1, v2), 0(digest)
	STP (v3, v4), 16(digest)

	BIC  $31, len
	MOVD len, ret+32(FP)
	RET
