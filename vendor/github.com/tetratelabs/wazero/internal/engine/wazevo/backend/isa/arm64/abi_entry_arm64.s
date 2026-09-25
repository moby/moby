//go:build arm64

#include "funcdata.h"
#include "textflag.h"

// See the comments on EmitGoEntryPreamble for what this function is supposed to do.
TEXT ·entrypoint(SB), NOSPLIT|NOFRAME, $0-48
	MOVD preambleExecutable+0(FP), R27
	MOVD functionExectuable+8(FP), R24
	MOVD executionContextPtr+16(FP), R0
	MOVD moduleContextPtr+24(FP), R1
	MOVD paramResultSlicePtr+32(FP), R19
	MOVD goAllocatedStackSlicePtr+40(FP), R26
	JMP  (R27)

TEXT ·afterGoFunctionCallEntrypoint(SB), NOSPLIT|NOFRAME, $0-32
	MOVD goCallReturnAddress+0(FP), R20
	MOVD executionContextPtr+8(FP), R0
	MOVD stackPointer+16(FP), R19

	// Save the current FP(R29), SP and LR(R30) into the wazevo.executionContext (stored in R0).
	MOVD R29, 16(R0) // Store FP(R29) into [RO, #ExecutionContextOffsets.OriginalFramePointer]
	MOVD RSP, R27    // Move SP to R27 (temporary register) since SP cannot be stored directly in str instructions.
	MOVD R27, 24(R0) // Store R27 into [RO, #ExecutionContextOffsets.OriginalFramePointer]
	MOVD R30, 32(R0) // Store R30 into [R0, #ExecutionContextOffsets.GoReturnAddress]

	// Load the new stack pointer (which sits somewhere in Go-allocated stack) into SP.
	MOVD R19, RSP
	JMP  (R20)
