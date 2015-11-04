//+build !noasm !appengine

// Copyright 2015, Klaus Post, see LICENSE for details.

// func crc32sse(a []byte) hash
TEXT ·crc32sse(SB),7, $0
    MOVQ    a+0(FP), R10
    XORQ    BX, BX
    // CRC32   dword (R10), EBX
    BYTE $0xF2; BYTE $0x41; BYTE $0x0f 
    BYTE $0x38; BYTE $0xf1; BYTE $0x1a


    // MOVL    (R10), AX
    // CRC32   EAX, EBX
    //BYTE $0xF2; BYTE $0x0f; 
    //BYTE $0x38; BYTE $0xf1; BYTE $0xd8

    MOVL    BX, ret+24(FP)
    RET

// func crc32sseAll(a []byte, dst []hash)
TEXT ·crc32sseAll(SB), 7, $0
    MOVQ    a+0(FP), R8
    MOVQ    a_len+8(FP), R10
    MOVQ    dst+24(FP), R9
    MOVQ    $0, AX
    SUBQ    $3, R10
    JZ      end
    JS      end
    MOVQ    R10, R13
    SHRQ    $2, R10  // len/4
    ANDQ    $3, R13  // len&3
    TESTQ   R10,R10
    JZ      remain_crc

crc_loop:
    MOVQ    (R8), R11
    XORQ    BX,BX
    XORQ    DX,DX
    XORQ    DI,DI
    MOVQ    R11, R12
    SHRQ    $8, R11
    MOVQ    R12, AX
    MOVQ    R11, CX
    SHRQ    $16, R12
    SHRQ    $16, R11
    MOVQ    R12, SI

    // CRC32   EAX, EBX
    BYTE $0xF2; BYTE $0x0f; 
    BYTE $0x38; BYTE $0xf1; BYTE $0xd8
    // CRC32   ECX, EDX
    BYTE $0xF2; BYTE $0x0f; 
    BYTE $0x38; BYTE $0xf1; BYTE $0xd1
    // CRC32   ESI, EDI
    BYTE $0xF2; BYTE $0x0f; 
    BYTE $0x38; BYTE $0xf1; BYTE $0xfe
    MOVL    BX, (R9)
    MOVL    DX, 4(R9)
    MOVL    DI, 8(R9)

    XORQ    BX, BX
    MOVL    R11, AX

    // CRC32   EAX, EBX
    BYTE $0xF2; BYTE $0x0f; 
    BYTE $0x38; BYTE $0xf1; BYTE $0xd8
    MOVL    BX, 12(R9)

    ADDQ    $16, R9
    ADDQ    $4, R8
    SUBQ    $1, R10
    JNZ     crc_loop

remain_crc:
    XORQ    BX, BX
    TESTQ    R13, R13
    JZ      end
rem_loop:
    MOVL    (R8), AX

    // CRC32   EAX, EBX
    BYTE $0xF2; BYTE $0x0f; 
    BYTE $0x38; BYTE $0xf1; BYTE $0xd8

    MOVL    BX,(R9)
    ADDQ    $4, R9
    ADDQ    $1, R8
    XORQ    BX, BX
    SUBQ    $1, R13
    JNZ    rem_loop
end:
    RET


// func matchLenSSE4(a, b []byte, max int) int
TEXT ·matchLenSSE4(SB), 7, $0
    MOVQ    a+0(FP),R8                  // R8: &a
    MOVQ    b+24(FP),R9                 // R9: &b
    MOVQ    max+48(FP), R10             // R10: max
    XORQ    R11, R11                    // match length

    MOVQ    R10, R12
    SHRQ    $4, R10                     // max/16
    ANDQ    $15, R12                    // max & 15
    CMPQ    R10, $0
    JEQ     matchlen_verysmall
loopback_matchlen:
    MOVOU   (R8),X0                     // a[x]
    MOVOU   (R9),X1                     // b[x]

    // PCMPESTRI $0x18, X1, X0
    BYTE $0x66; BYTE $0x0f; BYTE $0x3a
    BYTE $0x61; BYTE $0xc1; BYTE $0x18

    JC      match_ended

    ADDQ    $16, R8
    ADDQ    $16, R9
    ADDQ    $16, R11

    SUBQ    $1, R10
    JNZ     loopback_matchlen

matchlen_verysmall:
    CMPQ    R12 ,$0
    JEQ     done_matchlen
loopback_matchlen_single:
    // Naiive, but small use
    MOVB   (R8), R13
    MOVB   (R9), R14
    CMPB   R13, R14
    JNE    done_matchlen
    ADDQ   $1, R8
    ADDQ   $1, R9
    ADDQ   $1, R11
    SUBQ   $1, R12
    JNZ loopback_matchlen_single
    MOVQ    R11, ret+56(FP)
    RET
match_ended:
    ADDQ    CX, R11
done_matchlen:
    MOVQ    R11, ret+56(FP)
    RET

// func histogram(b []byte, h []int32)
TEXT ·histogram(SB), 7, $0
    MOVQ    b+0(FP),SI                  // SI: &b
    MOVQ    b_len+8(FP),R9              // R9: len(b)
    MOVQ    h+24(FP), DI                // DI: Histogram
    XORQ    R10, R10
    TESTQ   R9, R9
    JZ end_hist

loop_hist:
    MOVB    (SI), R10
    ADDL    $1, (DI)(R10*4)

    ADDQ    $1, SI
    SUBQ    $1, R9
    JNZ     loop_hist

end_hist:
    RET
