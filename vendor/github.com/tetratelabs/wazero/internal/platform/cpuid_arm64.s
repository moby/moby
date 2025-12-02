//go:build gc

#include "textflag.h"

// lifted from github.com/golang/sys and cpu/cpu_arm64.s

// func getisar0() uint64
TEXT ·getisar0(SB), NOSPLIT, $0-8
	// get Instruction Set Attributes 0 into x0
	// mrs x0, ID_AA64ISAR0_EL1 = d5380600
	WORD $0xd5380600
	MOVD R0, ret+0(FP)
	RET

// func getisar1() uint64
TEXT ·getisar1(SB), NOSPLIT, $0-8
	// get Instruction Set Attributes 1 into x0
	// mrs x0, ID_AA64ISAR1_EL1 = d5380620
	WORD $0xd5380620
	MOVD R0, ret+0(FP)
	RET
