#include "funcdata.h"
#include "textflag.h"

// entrypoint(preambleExecutable, functionExecutable *byte, executionContextPtr uintptr, moduleContextPtr *byte, paramResultPtr *uint64, goAllocatedStackSlicePtr uintptr
TEXT ·entrypoint(SB), NOSPLIT|NOFRAME, $0-48
	MOVQ preambleExecutable+0(FP), R11
	MOVQ functionExectuable+8(FP), R14
	MOVQ executionContextPtr+16(FP), AX       // First argument is passed in AX.
	MOVQ moduleContextPtr+24(FP), BX          // Second argument is passed in BX.
	MOVQ paramResultSlicePtr+32(FP), R12
	MOVQ goAllocatedStackSlicePtr+40(FP), R13
	JMP  R11

// afterGoFunctionCallEntrypoint(executable *byte, executionContextPtr uintptr, stackPointer, framePointer uintptr)
TEXT ·afterGoFunctionCallEntrypoint(SB), NOSPLIT|NOFRAME, $0-32
	MOVQ executable+0(FP), CX
	MOVQ executionContextPtr+8(FP), AX // First argument is passed in AX.

	// Save the stack pointer and frame pointer.
	MOVQ BP, 16(AX) // 16 == ExecutionContextOffsetOriginalFramePointer
	MOVQ SP, 24(AX) // 24 == ExecutionContextOffsetOriginalStackPointer

	// Then set the stack pointer and frame pointer to the values we got from the Go runtime.
	MOVQ framePointer+24(FP), BP

	// WARNING: do not update SP before BP, because the Go translates (FP) as (SP) + 8.
	MOVQ stackPointer+16(FP), SP

	JMP CX
