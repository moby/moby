//go:build amd64
// +build amd64

#include "textflag.h"


TEXT ·cpuid(SB),NOSPLIT,$0-24	
	MOVL ax+0(FP),AX
	MOVL cx+4(FP), CX
	CPUID
	MOVL AX,eax+8(FP)
	MOVL BX,ebx+12(FP)
	MOVL CX,ecx+16(FP)
	MOVL DX,edx+20(FP)
	RET

TEXT ·xgetbv(SB),NOSPLIT,$0-8
	XORL    CX, CX
	XGETBV
	MOVL AX, eax+0(FP)
	RET
