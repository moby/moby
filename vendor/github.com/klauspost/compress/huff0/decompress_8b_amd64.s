// +build !appengine
// +build gc
// +build !noasm

#include "textflag.h"
#include "funcdata.h"
#include "go_asm.h"

#define bufoff      256 // see decompress.go, we're using [4][256]byte table

// func decompress4x_main_loop_x86(pbr0, pbr1, pbr2, pbr3 *bitReaderShifted,
//	peekBits uint8, buf *byte, tbl *dEntrySingle) (int, bool)
TEXT ·decompress4x_8b_loop_x86(SB), NOSPLIT, $8
#define off             R8
#define buffer          DI
#define table           SI

#define br_bits_read    R9
#define br_value        R10
#define br_offset       R11
#define peek_bits       R12
#define exhausted       DX

#define br0             R13
#define br1             R14
#define br2             R15
#define br3             BP

	MOVQ BP, 0(SP)

	XORQ exhausted, exhausted // exhausted = false
	XORQ off, off             // off = 0

	MOVBQZX peekBits+32(FP), peek_bits
	MOVQ    buf+40(FP), buffer
	MOVQ    tbl+48(FP), table

	MOVQ pbr0+0(FP), br0
	MOVQ pbr1+8(FP), br1
	MOVQ pbr2+16(FP), br2
	MOVQ pbr3+24(FP), br3

main_loop:

	// const stream = 0
	// br0.fillFast()
	MOVBQZX bitReaderShifted_bitsRead(br0), br_bits_read
	MOVQ    bitReaderShifted_value(br0), br_value
	MOVQ    bitReaderShifted_off(br0), br_offset

	// if b.bitsRead >= 32 {
	CMPQ br_bits_read, $32
	JB   skip_fill0

	SUBQ $32, br_bits_read // b.bitsRead -= 32
	SUBQ $4, br_offset     // b.off -= 4

	// v := b.in[b.off-4 : b.off]
	// v = v[:4]
	// low := (uint32(v[0])) | (uint32(v[1]) << 8) | (uint32(v[2]) << 16) | (uint32(v[3]) << 24)
	MOVQ bitReaderShifted_in(br0), AX
	MOVL 0(br_offset)(AX*1), AX       // AX = uint32(b.in[b.off:b.off+4])

	// b.value |= uint64(low) << (b.bitsRead & 63)
	MOVQ br_bits_read, CX
	SHLQ CL, AX
	ORQ  AX, br_value

	// exhausted = exhausted || (br0.off < 4)
	CMPQ  br_offset, $4
	SETLT DL
	ORB   DL, DH

	// }
skip_fill0:

	// val0 := br0.peekTopBits(peekBits)
	MOVQ br_value, AX
	MOVQ peek_bits, CX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v0 := table[val0&mask]
	MOVW 0(table)(AX*2), AX // AX - v0

	// br0.advance(uint8(v0.entry))
	MOVB    AH, BL           // BL = uint8(v0.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CL, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// val1 := br0.peekTopBits(peekBits)
	MOVQ peek_bits, CX
	MOVQ br_value, AX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v1 := table[val1&mask]
	MOVW 0(table)(AX*2), AX // AX - v1

	// br0.advance(uint8(v1.entry))
	MOVB    AH, BH           // BH = uint8(v1.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CX, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// these two writes get coalesced
	// buf[stream][off] = uint8(v0.entry >> 8)
	// buf[stream][off+1] = uint8(v1.entry >> 8)
	MOVW BX, 0(buffer)(off*1)

	// SECOND PART:
	// val2 := br0.peekTopBits(peekBits)
	MOVQ br_value, AX
	MOVQ peek_bits, CX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v2 := table[val0&mask]
	MOVW 0(table)(AX*2), AX // AX - v0

	// br0.advance(uint8(v0.entry))
	MOVB    AH, BL           // BL = uint8(v0.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CL, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// val3 := br0.peekTopBits(peekBits)
	MOVQ peek_bits, CX
	MOVQ br_value, AX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v3 := table[val1&mask]
	MOVW 0(table)(AX*2), AX // AX - v1

	// br0.advance(uint8(v1.entry))
	MOVB    AH, BH           // BH = uint8(v1.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CX, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// these two writes get coalesced
	// buf[stream][off+2] = uint8(v2.entry >> 8)
	// buf[stream][off+3] = uint8(v3.entry >> 8)
	MOVW BX, 0+2(buffer)(off*1)

	// update the bitrader reader structure
	MOVB br_bits_read, bitReaderShifted_bitsRead(br0)
	MOVQ br_value, bitReaderShifted_value(br0)
	MOVQ br_offset, bitReaderShifted_off(br0)

	// const stream = 1
	// br1.fillFast()
	MOVBQZX bitReaderShifted_bitsRead(br1), br_bits_read
	MOVQ    bitReaderShifted_value(br1), br_value
	MOVQ    bitReaderShifted_off(br1), br_offset

	// if b.bitsRead >= 32 {
	CMPQ br_bits_read, $32
	JB   skip_fill1

	SUBQ $32, br_bits_read // b.bitsRead -= 32
	SUBQ $4, br_offset     // b.off -= 4

	// v := b.in[b.off-4 : b.off]
	// v = v[:4]
	// low := (uint32(v[0])) | (uint32(v[1]) << 8) | (uint32(v[2]) << 16) | (uint32(v[3]) << 24)
	MOVQ bitReaderShifted_in(br1), AX
	MOVL 0(br_offset)(AX*1), AX       // AX = uint32(b.in[b.off:b.off+4])

	// b.value |= uint64(low) << (b.bitsRead & 63)
	MOVQ br_bits_read, CX
	SHLQ CL, AX
	ORQ  AX, br_value

	// exhausted = exhausted || (br1.off < 4)
	CMPQ  br_offset, $4
	SETLT DL
	ORB   DL, DH

	// }
skip_fill1:

	// val0 := br1.peekTopBits(peekBits)
	MOVQ br_value, AX
	MOVQ peek_bits, CX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v0 := table[val0&mask]
	MOVW 0(table)(AX*2), AX // AX - v0

	// br1.advance(uint8(v0.entry))
	MOVB    AH, BL           // BL = uint8(v0.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CL, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// val1 := br1.peekTopBits(peekBits)
	MOVQ peek_bits, CX
	MOVQ br_value, AX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v1 := table[val1&mask]
	MOVW 0(table)(AX*2), AX // AX - v1

	// br1.advance(uint8(v1.entry))
	MOVB    AH, BH           // BH = uint8(v1.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CX, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// these two writes get coalesced
	// buf[stream][off] = uint8(v0.entry >> 8)
	// buf[stream][off+1] = uint8(v1.entry >> 8)
	MOVW BX, 256(buffer)(off*1)

	// SECOND PART:
	// val2 := br1.peekTopBits(peekBits)
	MOVQ br_value, AX
	MOVQ peek_bits, CX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v2 := table[val0&mask]
	MOVW 0(table)(AX*2), AX // AX - v0

	// br1.advance(uint8(v0.entry))
	MOVB    AH, BL           // BL = uint8(v0.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CL, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// val3 := br1.peekTopBits(peekBits)
	MOVQ peek_bits, CX
	MOVQ br_value, AX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v3 := table[val1&mask]
	MOVW 0(table)(AX*2), AX // AX - v1

	// br1.advance(uint8(v1.entry))
	MOVB    AH, BH           // BH = uint8(v1.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CX, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// these two writes get coalesced
	// buf[stream][off+2] = uint8(v2.entry >> 8)
	// buf[stream][off+3] = uint8(v3.entry >> 8)
	MOVW BX, 256+2(buffer)(off*1)

	// update the bitrader reader structure
	MOVB br_bits_read, bitReaderShifted_bitsRead(br1)
	MOVQ br_value, bitReaderShifted_value(br1)
	MOVQ br_offset, bitReaderShifted_off(br1)

	// const stream = 2
	// br2.fillFast()
	MOVBQZX bitReaderShifted_bitsRead(br2), br_bits_read
	MOVQ    bitReaderShifted_value(br2), br_value
	MOVQ    bitReaderShifted_off(br2), br_offset

	// if b.bitsRead >= 32 {
	CMPQ br_bits_read, $32
	JB   skip_fill2

	SUBQ $32, br_bits_read // b.bitsRead -= 32
	SUBQ $4, br_offset     // b.off -= 4

	// v := b.in[b.off-4 : b.off]
	// v = v[:4]
	// low := (uint32(v[0])) | (uint32(v[1]) << 8) | (uint32(v[2]) << 16) | (uint32(v[3]) << 24)
	MOVQ bitReaderShifted_in(br2), AX
	MOVL 0(br_offset)(AX*1), AX       // AX = uint32(b.in[b.off:b.off+4])

	// b.value |= uint64(low) << (b.bitsRead & 63)
	MOVQ br_bits_read, CX
	SHLQ CL, AX
	ORQ  AX, br_value

	// exhausted = exhausted || (br2.off < 4)
	CMPQ  br_offset, $4
	SETLT DL
	ORB   DL, DH

	// }
skip_fill2:

	// val0 := br2.peekTopBits(peekBits)
	MOVQ br_value, AX
	MOVQ peek_bits, CX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v0 := table[val0&mask]
	MOVW 0(table)(AX*2), AX // AX - v0

	// br2.advance(uint8(v0.entry))
	MOVB    AH, BL           // BL = uint8(v0.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CL, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// val1 := br2.peekTopBits(peekBits)
	MOVQ peek_bits, CX
	MOVQ br_value, AX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v1 := table[val1&mask]
	MOVW 0(table)(AX*2), AX // AX - v1

	// br2.advance(uint8(v1.entry))
	MOVB    AH, BH           // BH = uint8(v1.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CX, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// these two writes get coalesced
	// buf[stream][off] = uint8(v0.entry >> 8)
	// buf[stream][off+1] = uint8(v1.entry >> 8)
	MOVW BX, 512(buffer)(off*1)

	// SECOND PART:
	// val2 := br2.peekTopBits(peekBits)
	MOVQ br_value, AX
	MOVQ peek_bits, CX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v2 := table[val0&mask]
	MOVW 0(table)(AX*2), AX // AX - v0

	// br2.advance(uint8(v0.entry))
	MOVB    AH, BL           // BL = uint8(v0.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CL, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// val3 := br2.peekTopBits(peekBits)
	MOVQ peek_bits, CX
	MOVQ br_value, AX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v3 := table[val1&mask]
	MOVW 0(table)(AX*2), AX // AX - v1

	// br2.advance(uint8(v1.entry))
	MOVB    AH, BH           // BH = uint8(v1.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CX, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// these two writes get coalesced
	// buf[stream][off+2] = uint8(v2.entry >> 8)
	// buf[stream][off+3] = uint8(v3.entry >> 8)
	MOVW BX, 512+2(buffer)(off*1)

	// update the bitrader reader structure
	MOVB br_bits_read, bitReaderShifted_bitsRead(br2)
	MOVQ br_value, bitReaderShifted_value(br2)
	MOVQ br_offset, bitReaderShifted_off(br2)

	// const stream = 3
	// br3.fillFast()
	MOVBQZX bitReaderShifted_bitsRead(br3), br_bits_read
	MOVQ    bitReaderShifted_value(br3), br_value
	MOVQ    bitReaderShifted_off(br3), br_offset

	// if b.bitsRead >= 32 {
	CMPQ br_bits_read, $32
	JB   skip_fill3

	SUBQ $32, br_bits_read // b.bitsRead -= 32
	SUBQ $4, br_offset     // b.off -= 4

	// v := b.in[b.off-4 : b.off]
	// v = v[:4]
	// low := (uint32(v[0])) | (uint32(v[1]) << 8) | (uint32(v[2]) << 16) | (uint32(v[3]) << 24)
	MOVQ bitReaderShifted_in(br3), AX
	MOVL 0(br_offset)(AX*1), AX       // AX = uint32(b.in[b.off:b.off+4])

	// b.value |= uint64(low) << (b.bitsRead & 63)
	MOVQ br_bits_read, CX
	SHLQ CL, AX
	ORQ  AX, br_value

	// exhausted = exhausted || (br3.off < 4)
	CMPQ  br_offset, $4
	SETLT DL
	ORB   DL, DH

	// }
skip_fill3:

	// val0 := br3.peekTopBits(peekBits)
	MOVQ br_value, AX
	MOVQ peek_bits, CX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v0 := table[val0&mask]
	MOVW 0(table)(AX*2), AX // AX - v0

	// br3.advance(uint8(v0.entry))
	MOVB    AH, BL           // BL = uint8(v0.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CL, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// val1 := br3.peekTopBits(peekBits)
	MOVQ peek_bits, CX
	MOVQ br_value, AX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v1 := table[val1&mask]
	MOVW 0(table)(AX*2), AX // AX - v1

	// br3.advance(uint8(v1.entry))
	MOVB    AH, BH           // BH = uint8(v1.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CX, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// these two writes get coalesced
	// buf[stream][off] = uint8(v0.entry >> 8)
	// buf[stream][off+1] = uint8(v1.entry >> 8)
	MOVW BX, 768(buffer)(off*1)

	// SECOND PART:
	// val2 := br3.peekTopBits(peekBits)
	MOVQ br_value, AX
	MOVQ peek_bits, CX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v2 := table[val0&mask]
	MOVW 0(table)(AX*2), AX // AX - v0

	// br3.advance(uint8(v0.entry))
	MOVB    AH, BL           // BL = uint8(v0.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CL, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// val3 := br3.peekTopBits(peekBits)
	MOVQ peek_bits, CX
	MOVQ br_value, AX
	SHRQ CL, AX        // AX = (value >> peek_bits) & mask

	// v3 := table[val1&mask]
	MOVW 0(table)(AX*2), AX // AX - v1

	// br3.advance(uint8(v1.entry))
	MOVB    AH, BH           // BH = uint8(v1.entry >> 8)
	MOVBQZX AL, CX
	SHLQ    CX, br_value     // value <<= n
	ADDQ    CX, br_bits_read // bits_read += n

	// these two writes get coalesced
	// buf[stream][off+2] = uint8(v2.entry >> 8)
	// buf[stream][off+3] = uint8(v3.entry >> 8)
	MOVW BX, 768+2(buffer)(off*1)

	// update the bitrader reader structure
	MOVB br_bits_read, bitReaderShifted_bitsRead(br3)
	MOVQ br_value, bitReaderShifted_value(br3)
	MOVQ br_offset, bitReaderShifted_off(br3)

	ADDQ $4, off // off += 2

	TESTB DH, DH // any br[i].ofs < 4?
	JNZ   end

	CMPQ off, $bufoff
	JL   main_loop

end:
	MOVQ 0(SP), BP

	MOVB off, ret+56(FP)
	RET

#undef off
#undef buffer
#undef table

#undef br_bits_read
#undef br_value
#undef br_offset
#undef peek_bits
#undef exhausted

#undef br0
#undef br1
#undef br2
#undef br3
