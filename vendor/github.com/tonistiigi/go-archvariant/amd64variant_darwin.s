//go:build amd64 && darwin
// +build amd64,darwin

#include "textflag.h"

// Based on https://github.com/golang/sys/blob/ae416a5f93c7892a9dce2d607fc2479eabbacd70/cpu/cpu_x86.s#L31

// Note that Go currently has problems with AVX512 on darwin due to signal handling conflict in kernel and
// has been disabled in some libraries for runtime detection. We keep it on as this is unlikely to be used
// in runtime checks. golang/go#49233

// func darwinSupportsAVX512() bool
TEXT ·darwinSupportsAVX512(SB), NOSPLIT, $0-1
    MOVB    $0, ret+0(FP) // default to false
// These values from:
// https://github.com/apple/darwin-xnu/blob/xnu-4570.1.46/osfmk/i386/cpu_capabilities.h
#define commpage64_base_address         0x00007fffffe00000
#define commpage64_cpu_capabilities64   (commpage64_base_address+0x010)
#define commpage64_version              (commpage64_base_address+0x01E)
#define hasAVX512F                      0x0000004000000000
    MOVQ    $commpage64_version, BX
    CMPW    (BX), $13  // cpu_capabilities64 undefined in versions < 13
    JL      no_avx512
    MOVQ    $commpage64_cpu_capabilities64, BX
    MOVQ    $hasAVX512F, CX
    TESTQ   (BX), CX
    JZ      no_avx512
    MOVB    $1, ret+0(FP)
no_avx512:
    RET
