package frontend

import (
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"strings"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

type (
	// loweringState is used to keep the state of lowering.
	loweringState struct {
		// values holds the values on the Wasm stack.
		values           []ssa.Value
		controlFrames    []controlFrame
		unreachable      bool
		unreachableDepth int
		tmpForBrTable    []uint32
		pc               int
	}
	controlFrame struct {
		kind controlFrameKind
		// originalStackLen holds the number of values on the Wasm stack
		// when start executing this control frame minus params for the block.
		originalStackLenWithoutParam int
		// blk is the loop header if this is loop, and is the else-block if this is an if frame.
		blk,
		// followingBlock is the basic block we enter if we reach "end" of block.
		followingBlock ssa.BasicBlock
		blockType *wasm.FunctionType
		// clonedArgs hold the arguments to Else block.
		clonedArgs ssa.Values
	}

	controlFrameKind byte
)

// String implements fmt.Stringer for debugging.
func (l *loweringState) String() string {
	var str []string
	for _, v := range l.values {
		str = append(str, fmt.Sprintf("v%v", v.ID()))
	}
	var frames []string
	for i := range l.controlFrames {
		frames = append(frames, l.controlFrames[i].kind.String())
	}
	return fmt.Sprintf("\n\tunreachable=%v(depth=%d)\n\tstack: %s\n\tcontrol frames: %s",
		l.unreachable, l.unreachableDepth,
		strings.Join(str, ", "),
		strings.Join(frames, ", "),
	)
}

const (
	controlFrameKindFunction = iota + 1
	controlFrameKindLoop
	controlFrameKindIfWithElse
	controlFrameKindIfWithoutElse
	controlFrameKindBlock
)

// String implements fmt.Stringer for debugging.
func (k controlFrameKind) String() string {
	switch k {
	case controlFrameKindFunction:
		return "function"
	case controlFrameKindLoop:
		return "loop"
	case controlFrameKindIfWithElse:
		return "if_with_else"
	case controlFrameKindIfWithoutElse:
		return "if_without_else"
	case controlFrameKindBlock:
		return "block"
	default:
		panic(k)
	}
}

// isLoop returns true if this is a loop frame.
func (ctrl *controlFrame) isLoop() bool {
	return ctrl.kind == controlFrameKindLoop
}

// reset resets the state of loweringState for reuse.
func (l *loweringState) reset() {
	l.values = l.values[:0]
	l.controlFrames = l.controlFrames[:0]
	l.pc = 0
	l.unreachable = false
	l.unreachableDepth = 0
}

func (l *loweringState) peek() (ret ssa.Value) {
	tail := len(l.values) - 1
	return l.values[tail]
}

func (l *loweringState) pop() (ret ssa.Value) {
	tail := len(l.values) - 1
	ret = l.values[tail]
	l.values = l.values[:tail]
	return
}

func (l *loweringState) push(ret ssa.Value) {
	l.values = append(l.values, ret)
}

func (c *Compiler) nPeekDup(n int) ssa.Values {
	if n == 0 {
		return ssa.ValuesNil
	}

	l := c.state()
	tail := len(l.values)

	args := c.allocateVarLengthValues(n)
	args = args.Append(c.ssaBuilder.VarLengthPool(), l.values[tail-n:tail]...)
	return args
}

func (l *loweringState) ctrlPop() (ret controlFrame) {
	tail := len(l.controlFrames) - 1
	ret = l.controlFrames[tail]
	l.controlFrames = l.controlFrames[:tail]
	return
}

func (l *loweringState) ctrlPush(ret controlFrame) {
	l.controlFrames = append(l.controlFrames, ret)
}

func (l *loweringState) ctrlPeekAt(n int) (ret *controlFrame) {
	tail := len(l.controlFrames) - 1
	return &l.controlFrames[tail-n]
}

// lowerBody lowers the body of the Wasm function to the SSA form.
func (c *Compiler) lowerBody(entryBlk ssa.BasicBlock) {
	c.ssaBuilder.Seal(entryBlk)

	if c.needListener {
		c.callListenerBefore()
	}

	// Pushes the empty control frame which corresponds to the function return.
	c.loweringState.ctrlPush(controlFrame{
		kind:           controlFrameKindFunction,
		blockType:      c.wasmFunctionTyp,
		followingBlock: c.ssaBuilder.ReturnBlock(),
	})

	for c.loweringState.pc < len(c.wasmFunctionBody) {
		blkBeforeLowering := c.ssaBuilder.CurrentBlock()
		c.lowerCurrentOpcode()
		blkAfterLowering := c.ssaBuilder.CurrentBlock()
		if blkBeforeLowering != blkAfterLowering {
			// In Wasm, once a block exits, that means we've done compiling the block.
			// Therefore, we finalize the known bounds at the end of the block for the exiting block.
			c.finalizeKnownSafeBoundsAtTheEndOfBlock(blkBeforeLowering.ID())
			// After that, we initialize the known bounds for the new compilation target block.
			c.initializeCurrentBlockKnownBounds()
		}
	}
}

func (c *Compiler) state() *loweringState {
	return &c.loweringState
}

func (c *Compiler) lowerCurrentOpcode() {
	op := c.wasmFunctionBody[c.loweringState.pc]

	if c.needSourceOffsetInfo {
		c.ssaBuilder.SetCurrentSourceOffset(
			ssa.SourceOffset(c.loweringState.pc) + ssa.SourceOffset(c.wasmFunctionBodyOffsetInCodeSection),
		)
	}

	builder := c.ssaBuilder
	state := c.state()
	switch op {
	case wasm.OpcodeI32Const:
		c := c.readI32s()
		if state.unreachable {
			break
		}

		iconst := builder.AllocateInstruction().AsIconst32(uint32(c)).Insert(builder)
		value := iconst.Return()
		state.push(value)
	case wasm.OpcodeI64Const:
		c := c.readI64s()
		if state.unreachable {
			break
		}
		iconst := builder.AllocateInstruction().AsIconst64(uint64(c)).Insert(builder)
		value := iconst.Return()
		state.push(value)
	case wasm.OpcodeF32Const:
		f32 := c.readF32()
		if state.unreachable {
			break
		}
		f32const := builder.AllocateInstruction().
			AsF32const(f32).
			Insert(builder).
			Return()
		state.push(f32const)
	case wasm.OpcodeF64Const:
		f64 := c.readF64()
		if state.unreachable {
			break
		}
		f64const := builder.AllocateInstruction().
			AsF64const(f64).
			Insert(builder).
			Return()
		state.push(f64const)
	case wasm.OpcodeI32Add, wasm.OpcodeI64Add:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		iadd := builder.AllocateInstruction()
		iadd.AsIadd(x, y)
		builder.InsertInstruction(iadd)
		value := iadd.Return()
		state.push(value)
	case wasm.OpcodeI32Sub, wasm.OpcodeI64Sub:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		isub := builder.AllocateInstruction()
		isub.AsIsub(x, y)
		builder.InsertInstruction(isub)
		value := isub.Return()
		state.push(value)
	case wasm.OpcodeF32Add, wasm.OpcodeF64Add:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		iadd := builder.AllocateInstruction()
		iadd.AsFadd(x, y)
		builder.InsertInstruction(iadd)
		value := iadd.Return()
		state.push(value)
	case wasm.OpcodeI32Mul, wasm.OpcodeI64Mul:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		imul := builder.AllocateInstruction()
		imul.AsImul(x, y)
		builder.InsertInstruction(imul)
		value := imul.Return()
		state.push(value)
	case wasm.OpcodeF32Sub, wasm.OpcodeF64Sub:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		isub := builder.AllocateInstruction()
		isub.AsFsub(x, y)
		builder.InsertInstruction(isub)
		value := isub.Return()
		state.push(value)
	case wasm.OpcodeF32Mul, wasm.OpcodeF64Mul:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		isub := builder.AllocateInstruction()
		isub.AsFmul(x, y)
		builder.InsertInstruction(isub)
		value := isub.Return()
		state.push(value)
	case wasm.OpcodeF32Div, wasm.OpcodeF64Div:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		isub := builder.AllocateInstruction()
		isub.AsFdiv(x, y)
		builder.InsertInstruction(isub)
		value := isub.Return()
		state.push(value)
	case wasm.OpcodeF32Max, wasm.OpcodeF64Max:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		isub := builder.AllocateInstruction()
		isub.AsFmax(x, y)
		builder.InsertInstruction(isub)
		value := isub.Return()
		state.push(value)
	case wasm.OpcodeF32Min, wasm.OpcodeF64Min:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		isub := builder.AllocateInstruction()
		isub.AsFmin(x, y)
		builder.InsertInstruction(isub)
		value := isub.Return()
		state.push(value)
	case wasm.OpcodeI64Extend8S:
		if state.unreachable {
			break
		}
		c.insertIntegerExtend(true, 8, 64)
	case wasm.OpcodeI64Extend16S:
		if state.unreachable {
			break
		}
		c.insertIntegerExtend(true, 16, 64)
	case wasm.OpcodeI64Extend32S, wasm.OpcodeI64ExtendI32S:
		if state.unreachable {
			break
		}
		c.insertIntegerExtend(true, 32, 64)
	case wasm.OpcodeI64ExtendI32U:
		if state.unreachable {
			break
		}
		c.insertIntegerExtend(false, 32, 64)
	case wasm.OpcodeI32Extend8S:
		if state.unreachable {
			break
		}
		c.insertIntegerExtend(true, 8, 32)
	case wasm.OpcodeI32Extend16S:
		if state.unreachable {
			break
		}
		c.insertIntegerExtend(true, 16, 32)
	case wasm.OpcodeI32Eqz, wasm.OpcodeI64Eqz:
		if state.unreachable {
			break
		}
		x := state.pop()
		zero := builder.AllocateInstruction()
		if op == wasm.OpcodeI32Eqz {
			zero.AsIconst32(0)
		} else {
			zero.AsIconst64(0)
		}
		builder.InsertInstruction(zero)
		icmp := builder.AllocateInstruction().
			AsIcmp(x, zero.Return(), ssa.IntegerCmpCondEqual).
			Insert(builder).
			Return()
		state.push(icmp)
	case wasm.OpcodeI32Eq, wasm.OpcodeI64Eq:
		if state.unreachable {
			break
		}
		c.insertIcmp(ssa.IntegerCmpCondEqual)
	case wasm.OpcodeI32Ne, wasm.OpcodeI64Ne:
		if state.unreachable {
			break
		}
		c.insertIcmp(ssa.IntegerCmpCondNotEqual)
	case wasm.OpcodeI32LtS, wasm.OpcodeI64LtS:
		if state.unreachable {
			break
		}
		c.insertIcmp(ssa.IntegerCmpCondSignedLessThan)
	case wasm.OpcodeI32LtU, wasm.OpcodeI64LtU:
		if state.unreachable {
			break
		}
		c.insertIcmp(ssa.IntegerCmpCondUnsignedLessThan)
	case wasm.OpcodeI32GtS, wasm.OpcodeI64GtS:
		if state.unreachable {
			break
		}
		c.insertIcmp(ssa.IntegerCmpCondSignedGreaterThan)
	case wasm.OpcodeI32GtU, wasm.OpcodeI64GtU:
		if state.unreachable {
			break
		}
		c.insertIcmp(ssa.IntegerCmpCondUnsignedGreaterThan)
	case wasm.OpcodeI32LeS, wasm.OpcodeI64LeS:
		if state.unreachable {
			break
		}
		c.insertIcmp(ssa.IntegerCmpCondSignedLessThanOrEqual)
	case wasm.OpcodeI32LeU, wasm.OpcodeI64LeU:
		if state.unreachable {
			break
		}
		c.insertIcmp(ssa.IntegerCmpCondUnsignedLessThanOrEqual)
	case wasm.OpcodeI32GeS, wasm.OpcodeI64GeS:
		if state.unreachable {
			break
		}
		c.insertIcmp(ssa.IntegerCmpCondSignedGreaterThanOrEqual)
	case wasm.OpcodeI32GeU, wasm.OpcodeI64GeU:
		if state.unreachable {
			break
		}
		c.insertIcmp(ssa.IntegerCmpCondUnsignedGreaterThanOrEqual)

	case wasm.OpcodeF32Eq, wasm.OpcodeF64Eq:
		if state.unreachable {
			break
		}
		c.insertFcmp(ssa.FloatCmpCondEqual)
	case wasm.OpcodeF32Ne, wasm.OpcodeF64Ne:
		if state.unreachable {
			break
		}
		c.insertFcmp(ssa.FloatCmpCondNotEqual)
	case wasm.OpcodeF32Lt, wasm.OpcodeF64Lt:
		if state.unreachable {
			break
		}
		c.insertFcmp(ssa.FloatCmpCondLessThan)
	case wasm.OpcodeF32Gt, wasm.OpcodeF64Gt:
		if state.unreachable {
			break
		}
		c.insertFcmp(ssa.FloatCmpCondGreaterThan)
	case wasm.OpcodeF32Le, wasm.OpcodeF64Le:
		if state.unreachable {
			break
		}
		c.insertFcmp(ssa.FloatCmpCondLessThanOrEqual)
	case wasm.OpcodeF32Ge, wasm.OpcodeF64Ge:
		if state.unreachable {
			break
		}
		c.insertFcmp(ssa.FloatCmpCondGreaterThanOrEqual)
	case wasm.OpcodeF32Neg, wasm.OpcodeF64Neg:
		if state.unreachable {
			break
		}
		x := state.pop()
		v := builder.AllocateInstruction().AsFneg(x).Insert(builder).Return()
		state.push(v)
	case wasm.OpcodeF32Sqrt, wasm.OpcodeF64Sqrt:
		if state.unreachable {
			break
		}
		x := state.pop()
		v := builder.AllocateInstruction().AsSqrt(x).Insert(builder).Return()
		state.push(v)
	case wasm.OpcodeF32Abs, wasm.OpcodeF64Abs:
		if state.unreachable {
			break
		}
		x := state.pop()
		v := builder.AllocateInstruction().AsFabs(x).Insert(builder).Return()
		state.push(v)
	case wasm.OpcodeF32Copysign, wasm.OpcodeF64Copysign:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		v := builder.AllocateInstruction().AsFcopysign(x, y).Insert(builder).Return()
		state.push(v)

	case wasm.OpcodeF32Ceil, wasm.OpcodeF64Ceil:
		if state.unreachable {
			break
		}
		x := state.pop()
		v := builder.AllocateInstruction().AsCeil(x).Insert(builder).Return()
		state.push(v)
	case wasm.OpcodeF32Floor, wasm.OpcodeF64Floor:
		if state.unreachable {
			break
		}
		x := state.pop()
		v := builder.AllocateInstruction().AsFloor(x).Insert(builder).Return()
		state.push(v)
	case wasm.OpcodeF32Trunc, wasm.OpcodeF64Trunc:
		if state.unreachable {
			break
		}
		x := state.pop()
		v := builder.AllocateInstruction().AsTrunc(x).Insert(builder).Return()
		state.push(v)
	case wasm.OpcodeF32Nearest, wasm.OpcodeF64Nearest:
		if state.unreachable {
			break
		}
		x := state.pop()
		v := builder.AllocateInstruction().AsNearest(x).Insert(builder).Return()
		state.push(v)
	case wasm.OpcodeI64TruncF64S, wasm.OpcodeI64TruncF32S,
		wasm.OpcodeI32TruncF64S, wasm.OpcodeI32TruncF32S,
		wasm.OpcodeI64TruncF64U, wasm.OpcodeI64TruncF32U,
		wasm.OpcodeI32TruncF64U, wasm.OpcodeI32TruncF32U:
		if state.unreachable {
			break
		}
		ret := builder.AllocateInstruction().AsFcvtToInt(
			state.pop(),
			c.execCtxPtrValue,
			op == wasm.OpcodeI64TruncF64S || op == wasm.OpcodeI64TruncF32S || op == wasm.OpcodeI32TruncF32S || op == wasm.OpcodeI32TruncF64S,
			op == wasm.OpcodeI64TruncF64S || op == wasm.OpcodeI64TruncF32S || op == wasm.OpcodeI64TruncF64U || op == wasm.OpcodeI64TruncF32U,
			false,
		).Insert(builder).Return()
		state.push(ret)
	case wasm.OpcodeMiscPrefix:
		state.pc++
		// A misc opcode is encoded as an unsigned variable 32-bit integer.
		miscOpUint, num, err := leb128.LoadUint32(c.wasmFunctionBody[state.pc:])
		if err != nil {
			// In normal conditions this should never happen because the function has passed validation.
			panic(fmt.Sprintf("failed to read misc opcode: %v", err))
		}
		state.pc += int(num - 1)
		miscOp := wasm.OpcodeMisc(miscOpUint)
		switch miscOp {
		case wasm.OpcodeMiscI64TruncSatF64S, wasm.OpcodeMiscI64TruncSatF32S,
			wasm.OpcodeMiscI32TruncSatF64S, wasm.OpcodeMiscI32TruncSatF32S,
			wasm.OpcodeMiscI64TruncSatF64U, wasm.OpcodeMiscI64TruncSatF32U,
			wasm.OpcodeMiscI32TruncSatF64U, wasm.OpcodeMiscI32TruncSatF32U:
			if state.unreachable {
				break
			}
			ret := builder.AllocateInstruction().AsFcvtToInt(
				state.pop(),
				c.execCtxPtrValue,
				miscOp == wasm.OpcodeMiscI64TruncSatF64S || miscOp == wasm.OpcodeMiscI64TruncSatF32S || miscOp == wasm.OpcodeMiscI32TruncSatF32S || miscOp == wasm.OpcodeMiscI32TruncSatF64S,
				miscOp == wasm.OpcodeMiscI64TruncSatF64S || miscOp == wasm.OpcodeMiscI64TruncSatF32S || miscOp == wasm.OpcodeMiscI64TruncSatF64U || miscOp == wasm.OpcodeMiscI64TruncSatF32U,
				true,
			).Insert(builder).Return()
			state.push(ret)

		case wasm.OpcodeMiscTableSize:
			tableIndex := c.readI32u()
			if state.unreachable {
				break
			}

			// Load the table.
			loadTableInstancePtr := builder.AllocateInstruction()
			loadTableInstancePtr.AsLoad(c.moduleCtxPtrValue, c.offset.TableOffset(int(tableIndex)).U32(), ssa.TypeI64)
			builder.InsertInstruction(loadTableInstancePtr)
			tableInstancePtr := loadTableInstancePtr.Return()

			// Load the table's length.
			loadTableLen := builder.AllocateInstruction().
				AsLoad(tableInstancePtr, tableInstanceLenOffset, ssa.TypeI32).
				Insert(builder)
			state.push(loadTableLen.Return())

		case wasm.OpcodeMiscTableGrow:
			tableIndex := c.readI32u()
			if state.unreachable {
				break
			}

			c.storeCallerModuleContext()

			tableIndexVal := builder.AllocateInstruction().AsIconst32(tableIndex).Insert(builder).Return()

			num := state.pop()
			r := state.pop()

			tableGrowPtr := builder.AllocateInstruction().
				AsLoad(c.execCtxPtrValue,
					wazevoapi.ExecutionContextOffsetTableGrowTrampolineAddress.U32(),
					ssa.TypeI64,
				).Insert(builder).Return()

			args := c.allocateVarLengthValues(4, c.execCtxPtrValue, tableIndexVal, num, r)
			callGrowRet := builder.
				AllocateInstruction().
				AsCallIndirect(tableGrowPtr, &c.tableGrowSig, args).
				Insert(builder).Return()
			state.push(callGrowRet)

		case wasm.OpcodeMiscTableCopy:
			dstTableIndex := c.readI32u()
			srcTableIndex := c.readI32u()
			if state.unreachable {
				break
			}

			copySize := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()
			srcOffset := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()
			dstOffset := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()

			// Out of bounds check.
			dstTableInstancePtr := c.boundsCheckInTable(dstTableIndex, dstOffset, copySize)
			srcTableInstancePtr := c.boundsCheckInTable(srcTableIndex, srcOffset, copySize)

			dstTableBaseAddr := c.loadTableBaseAddr(dstTableInstancePtr)
			srcTableBaseAddr := c.loadTableBaseAddr(srcTableInstancePtr)

			three := builder.AllocateInstruction().AsIconst64(3).Insert(builder).Return()

			dstOffsetInBytes := builder.AllocateInstruction().AsIshl(dstOffset, three).Insert(builder).Return()
			dstAddr := builder.AllocateInstruction().AsIadd(dstTableBaseAddr, dstOffsetInBytes).Insert(builder).Return()
			srcOffsetInBytes := builder.AllocateInstruction().AsIshl(srcOffset, three).Insert(builder).Return()
			srcAddr := builder.AllocateInstruction().AsIadd(srcTableBaseAddr, srcOffsetInBytes).Insert(builder).Return()

			copySizeInBytes := builder.AllocateInstruction().AsIshl(copySize, three).Insert(builder).Return()
			c.callMemmove(dstAddr, srcAddr, copySizeInBytes)

		case wasm.OpcodeMiscMemoryCopy:
			state.pc += 2 // +2 to skip two memory indexes which are fixed to zero.
			if state.unreachable {
				break
			}

			copySize := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()
			srcOffset := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()
			dstOffset := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()

			// Out of bounds check.
			memLen := c.getMemoryLenValue(false)
			c.boundsCheckInMemory(memLen, dstOffset, copySize)
			c.boundsCheckInMemory(memLen, srcOffset, copySize)

			memBase := c.getMemoryBaseValue(false)
			dstAddr := builder.AllocateInstruction().AsIadd(memBase, dstOffset).Insert(builder).Return()
			srcAddr := builder.AllocateInstruction().AsIadd(memBase, srcOffset).Insert(builder).Return()

			c.callMemmove(dstAddr, srcAddr, copySize)

		case wasm.OpcodeMiscTableFill:
			tableIndex := c.readI32u()
			if state.unreachable {
				break
			}
			fillSize := state.pop()
			value := state.pop()
			offset := state.pop()

			fillSizeExt := builder.
				AllocateInstruction().AsUExtend(fillSize, 32, 64).Insert(builder).Return()
			offsetExt := builder.
				AllocateInstruction().AsUExtend(offset, 32, 64).Insert(builder).Return()
			tableInstancePtr := c.boundsCheckInTable(tableIndex, offsetExt, fillSizeExt)

			three := builder.AllocateInstruction().AsIconst64(3).Insert(builder).Return()
			offsetInBytes := builder.AllocateInstruction().AsIshl(offsetExt, three).Insert(builder).Return()
			fillSizeInBytes := builder.AllocateInstruction().AsIshl(fillSizeExt, three).Insert(builder).Return()

			// Calculate the base address of the table.
			tableBaseAddr := c.loadTableBaseAddr(tableInstancePtr)
			addr := builder.AllocateInstruction().AsIadd(tableBaseAddr, offsetInBytes).Insert(builder).Return()

			// Prepare the loop and following block.
			beforeLoop := builder.AllocateBasicBlock()
			loopBlk := builder.AllocateBasicBlock()
			loopVar := loopBlk.AddParam(builder, ssa.TypeI64)
			followingBlk := builder.AllocateBasicBlock()

			// Uses the copy trick for faster filling buffer like memory.fill, but in this case we copy 8 bytes at a time.
			// 	buf := memoryInst.Buffer[offset : offset+fillSize]
			// 	buf[0:8] = value
			// 	for i := 8; i < fillSize; i *= 2 { Begin with 8 bytes.
			// 		copy(buf[i:], buf[:i])
			// 	}

			// Insert the jump to the beforeLoop block; If the fillSize is zero, then jump to the following block to skip entire logics.
			zero := builder.AllocateInstruction().AsIconst64(0).Insert(builder).Return()
			ifFillSizeZero := builder.AllocateInstruction().AsIcmp(fillSizeExt, zero, ssa.IntegerCmpCondEqual).
				Insert(builder).Return()
			builder.AllocateInstruction().AsBrnz(ifFillSizeZero, ssa.ValuesNil, followingBlk).Insert(builder)
			c.insertJumpToBlock(ssa.ValuesNil, beforeLoop)

			// buf[0:8] = value
			builder.SetCurrentBlock(beforeLoop)
			builder.AllocateInstruction().AsStore(ssa.OpcodeStore, value, addr, 0).Insert(builder)
			initValue := builder.AllocateInstruction().AsIconst64(8).Insert(builder).Return()
			c.insertJumpToBlock(c.allocateVarLengthValues(1, initValue), loopBlk)

			builder.SetCurrentBlock(loopBlk)
			dstAddr := builder.AllocateInstruction().AsIadd(addr, loopVar).Insert(builder).Return()

			// If loopVar*2 > fillSizeInBytes, then count must be fillSizeInBytes-loopVar.
			var count ssa.Value
			{
				loopVarDoubled := builder.AllocateInstruction().AsIadd(loopVar, loopVar).Insert(builder).Return()
				loopVarDoubledLargerThanFillSize := builder.
					AllocateInstruction().AsIcmp(loopVarDoubled, fillSizeInBytes, ssa.IntegerCmpCondUnsignedGreaterThanOrEqual).
					Insert(builder).Return()
				diff := builder.AllocateInstruction().AsIsub(fillSizeInBytes, loopVar).Insert(builder).Return()
				count = builder.AllocateInstruction().AsSelect(loopVarDoubledLargerThanFillSize, diff, loopVar).Insert(builder).Return()
			}

			c.callMemmove(dstAddr, addr, count)

			shiftAmount := builder.AllocateInstruction().AsIconst64(1).Insert(builder).Return()
			newLoopVar := builder.AllocateInstruction().AsIshl(loopVar, shiftAmount).Insert(builder).Return()
			loopVarLessThanFillSize := builder.AllocateInstruction().
				AsIcmp(newLoopVar, fillSizeInBytes, ssa.IntegerCmpCondUnsignedLessThan).Insert(builder).Return()

			builder.AllocateInstruction().
				AsBrnz(loopVarLessThanFillSize, c.allocateVarLengthValues(1, newLoopVar), loopBlk).
				Insert(builder)

			c.insertJumpToBlock(ssa.ValuesNil, followingBlk)
			builder.SetCurrentBlock(followingBlk)

			builder.Seal(beforeLoop)
			builder.Seal(loopBlk)
			builder.Seal(followingBlk)

		case wasm.OpcodeMiscMemoryFill:
			state.pc++ // Skip the memory index which is fixed to zero.
			if state.unreachable {
				break
			}

			fillSize := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()
			value := state.pop()
			offset := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()

			// Out of bounds check.
			c.boundsCheckInMemory(c.getMemoryLenValue(false), offset, fillSize)

			// Calculate the base address:
			addr := builder.AllocateInstruction().AsIadd(c.getMemoryBaseValue(false), offset).Insert(builder).Return()

			// Uses the copy trick for faster filling buffer: https://gist.github.com/taylorza/df2f89d5f9ab3ffd06865062a4cf015d
			// 	buf := memoryInst.Buffer[offset : offset+fillSize]
			// 	buf[0] = value
			// 	for i := 1; i < fillSize; i *= 2 {
			// 		copy(buf[i:], buf[:i])
			// 	}

			// Prepare the loop and following block.
			beforeLoop := builder.AllocateBasicBlock()
			loopBlk := builder.AllocateBasicBlock()
			loopVar := loopBlk.AddParam(builder, ssa.TypeI64)
			followingBlk := builder.AllocateBasicBlock()

			// Insert the jump to the beforeLoop block; If the fillSize is zero, then jump to the following block to skip entire logics.
			zero := builder.AllocateInstruction().AsIconst64(0).Insert(builder).Return()
			ifFillSizeZero := builder.AllocateInstruction().AsIcmp(fillSize, zero, ssa.IntegerCmpCondEqual).
				Insert(builder).Return()
			builder.AllocateInstruction().AsBrnz(ifFillSizeZero, ssa.ValuesNil, followingBlk).Insert(builder)
			c.insertJumpToBlock(ssa.ValuesNil, beforeLoop)

			// buf[0] = value
			builder.SetCurrentBlock(beforeLoop)
			builder.AllocateInstruction().AsStore(ssa.OpcodeIstore8, value, addr, 0).Insert(builder)
			initValue := builder.AllocateInstruction().AsIconst64(1).Insert(builder).Return()
			c.insertJumpToBlock(c.allocateVarLengthValues(1, initValue), loopBlk)

			builder.SetCurrentBlock(loopBlk)
			dstAddr := builder.AllocateInstruction().AsIadd(addr, loopVar).Insert(builder).Return()

			// If loopVar*2 > fillSizeExt, then count must be fillSizeExt-loopVar.
			var count ssa.Value
			{
				loopVarDoubled := builder.AllocateInstruction().AsIadd(loopVar, loopVar).Insert(builder).Return()
				loopVarDoubledLargerThanFillSize := builder.
					AllocateInstruction().AsIcmp(loopVarDoubled, fillSize, ssa.IntegerCmpCondUnsignedGreaterThanOrEqual).
					Insert(builder).Return()
				diff := builder.AllocateInstruction().AsIsub(fillSize, loopVar).Insert(builder).Return()
				count = builder.AllocateInstruction().AsSelect(loopVarDoubledLargerThanFillSize, diff, loopVar).Insert(builder).Return()
			}

			c.callMemmove(dstAddr, addr, count)

			shiftAmount := builder.AllocateInstruction().AsIconst64(1).Insert(builder).Return()
			newLoopVar := builder.AllocateInstruction().AsIshl(loopVar, shiftAmount).Insert(builder).Return()
			loopVarLessThanFillSize := builder.AllocateInstruction().
				AsIcmp(newLoopVar, fillSize, ssa.IntegerCmpCondUnsignedLessThan).Insert(builder).Return()

			builder.AllocateInstruction().
				AsBrnz(loopVarLessThanFillSize, c.allocateVarLengthValues(1, newLoopVar), loopBlk).
				Insert(builder)

			c.insertJumpToBlock(ssa.ValuesNil, followingBlk)
			builder.SetCurrentBlock(followingBlk)

			builder.Seal(beforeLoop)
			builder.Seal(loopBlk)
			builder.Seal(followingBlk)

		case wasm.OpcodeMiscMemoryInit:
			index := c.readI32u()
			state.pc++ // Skip the memory index which is fixed to zero.
			if state.unreachable {
				break
			}

			copySize := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()
			offsetInDataInstance := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()
			offsetInMemory := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()

			dataInstPtr := c.dataOrElementInstanceAddr(index, c.offset.DataInstances1stElement)

			// Bounds check.
			c.boundsCheckInMemory(c.getMemoryLenValue(false), offsetInMemory, copySize)
			c.boundsCheckInDataOrElementInstance(dataInstPtr, offsetInDataInstance, copySize, wazevoapi.ExitCodeMemoryOutOfBounds)

			dataInstBaseAddr := builder.AllocateInstruction().AsLoad(dataInstPtr, 0, ssa.TypeI64).Insert(builder).Return()
			srcAddr := builder.AllocateInstruction().AsIadd(dataInstBaseAddr, offsetInDataInstance).Insert(builder).Return()

			memBase := c.getMemoryBaseValue(false)
			dstAddr := builder.AllocateInstruction().AsIadd(memBase, offsetInMemory).Insert(builder).Return()

			c.callMemmove(dstAddr, srcAddr, copySize)

		case wasm.OpcodeMiscTableInit:
			elemIndex := c.readI32u()
			tableIndex := c.readI32u()
			if state.unreachable {
				break
			}

			copySize := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()
			offsetInElementInstance := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()
			offsetInTable := builder.
				AllocateInstruction().AsUExtend(state.pop(), 32, 64).Insert(builder).Return()

			elemInstPtr := c.dataOrElementInstanceAddr(elemIndex, c.offset.ElementInstances1stElement)

			// Bounds check.
			tableInstancePtr := c.boundsCheckInTable(tableIndex, offsetInTable, copySize)
			c.boundsCheckInDataOrElementInstance(elemInstPtr, offsetInElementInstance, copySize, wazevoapi.ExitCodeTableOutOfBounds)

			three := builder.AllocateInstruction().AsIconst64(3).Insert(builder).Return()
			// Calculates the destination address in the table.
			tableOffsetInBytes := builder.AllocateInstruction().AsIshl(offsetInTable, three).Insert(builder).Return()
			tableBaseAddr := c.loadTableBaseAddr(tableInstancePtr)
			dstAddr := builder.AllocateInstruction().AsIadd(tableBaseAddr, tableOffsetInBytes).Insert(builder).Return()

			// Calculates the source address in the element instance.
			srcOffsetInBytes := builder.AllocateInstruction().AsIshl(offsetInElementInstance, three).Insert(builder).Return()
			elemInstBaseAddr := builder.AllocateInstruction().AsLoad(elemInstPtr, 0, ssa.TypeI64).Insert(builder).Return()
			srcAddr := builder.AllocateInstruction().AsIadd(elemInstBaseAddr, srcOffsetInBytes).Insert(builder).Return()

			copySizeInBytes := builder.AllocateInstruction().AsIshl(copySize, three).Insert(builder).Return()
			c.callMemmove(dstAddr, srcAddr, copySizeInBytes)

		case wasm.OpcodeMiscElemDrop:
			index := c.readI32u()
			if state.unreachable {
				break
			}

			c.dropDataOrElementInstance(index, c.offset.ElementInstances1stElement)

		case wasm.OpcodeMiscDataDrop:
			index := c.readI32u()
			if state.unreachable {
				break
			}
			c.dropDataOrElementInstance(index, c.offset.DataInstances1stElement)

		default:
			panic("Unknown MiscOp " + wasm.MiscInstructionName(miscOp))
		}

	case wasm.OpcodeI32ReinterpretF32:
		if state.unreachable {
			break
		}
		reinterpret := builder.AllocateInstruction().
			AsBitcast(state.pop(), ssa.TypeI32).
			Insert(builder).Return()
		state.push(reinterpret)

	case wasm.OpcodeI64ReinterpretF64:
		if state.unreachable {
			break
		}
		reinterpret := builder.AllocateInstruction().
			AsBitcast(state.pop(), ssa.TypeI64).
			Insert(builder).Return()
		state.push(reinterpret)

	case wasm.OpcodeF32ReinterpretI32:
		if state.unreachable {
			break
		}
		reinterpret := builder.AllocateInstruction().
			AsBitcast(state.pop(), ssa.TypeF32).
			Insert(builder).Return()
		state.push(reinterpret)

	case wasm.OpcodeF64ReinterpretI64:
		if state.unreachable {
			break
		}
		reinterpret := builder.AllocateInstruction().
			AsBitcast(state.pop(), ssa.TypeF64).
			Insert(builder).Return()
		state.push(reinterpret)

	case wasm.OpcodeI32DivS, wasm.OpcodeI64DivS:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		result := builder.AllocateInstruction().AsSDiv(x, y, c.execCtxPtrValue).Insert(builder).Return()
		state.push(result)

	case wasm.OpcodeI32DivU, wasm.OpcodeI64DivU:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		result := builder.AllocateInstruction().AsUDiv(x, y, c.execCtxPtrValue).Insert(builder).Return()
		state.push(result)

	case wasm.OpcodeI32RemS, wasm.OpcodeI64RemS:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		result := builder.AllocateInstruction().AsSRem(x, y, c.execCtxPtrValue).Insert(builder).Return()
		state.push(result)

	case wasm.OpcodeI32RemU, wasm.OpcodeI64RemU:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		result := builder.AllocateInstruction().AsURem(x, y, c.execCtxPtrValue).Insert(builder).Return()
		state.push(result)

	case wasm.OpcodeI32And, wasm.OpcodeI64And:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		and := builder.AllocateInstruction()
		and.AsBand(x, y)
		builder.InsertInstruction(and)
		value := and.Return()
		state.push(value)
	case wasm.OpcodeI32Or, wasm.OpcodeI64Or:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		or := builder.AllocateInstruction()
		or.AsBor(x, y)
		builder.InsertInstruction(or)
		value := or.Return()
		state.push(value)
	case wasm.OpcodeI32Xor, wasm.OpcodeI64Xor:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		xor := builder.AllocateInstruction()
		xor.AsBxor(x, y)
		builder.InsertInstruction(xor)
		value := xor.Return()
		state.push(value)
	case wasm.OpcodeI32Shl, wasm.OpcodeI64Shl:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		ishl := builder.AllocateInstruction()
		ishl.AsIshl(x, y)
		builder.InsertInstruction(ishl)
		value := ishl.Return()
		state.push(value)
	case wasm.OpcodeI32ShrU, wasm.OpcodeI64ShrU:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		ishl := builder.AllocateInstruction()
		ishl.AsUshr(x, y)
		builder.InsertInstruction(ishl)
		value := ishl.Return()
		state.push(value)
	case wasm.OpcodeI32ShrS, wasm.OpcodeI64ShrS:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		ishl := builder.AllocateInstruction()
		ishl.AsSshr(x, y)
		builder.InsertInstruction(ishl)
		value := ishl.Return()
		state.push(value)
	case wasm.OpcodeI32Rotl, wasm.OpcodeI64Rotl:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		rotl := builder.AllocateInstruction()
		rotl.AsRotl(x, y)
		builder.InsertInstruction(rotl)
		value := rotl.Return()
		state.push(value)
	case wasm.OpcodeI32Rotr, wasm.OpcodeI64Rotr:
		if state.unreachable {
			break
		}
		y, x := state.pop(), state.pop()
		rotr := builder.AllocateInstruction()
		rotr.AsRotr(x, y)
		builder.InsertInstruction(rotr)
		value := rotr.Return()
		state.push(value)
	case wasm.OpcodeI32Clz, wasm.OpcodeI64Clz:
		if state.unreachable {
			break
		}
		x := state.pop()
		clz := builder.AllocateInstruction()
		clz.AsClz(x)
		builder.InsertInstruction(clz)
		value := clz.Return()
		state.push(value)
	case wasm.OpcodeI32Ctz, wasm.OpcodeI64Ctz:
		if state.unreachable {
			break
		}
		x := state.pop()
		ctz := builder.AllocateInstruction()
		ctz.AsCtz(x)
		builder.InsertInstruction(ctz)
		value := ctz.Return()
		state.push(value)
	case wasm.OpcodeI32Popcnt, wasm.OpcodeI64Popcnt:
		if state.unreachable {
			break
		}
		x := state.pop()
		popcnt := builder.AllocateInstruction()
		popcnt.AsPopcnt(x)
		builder.InsertInstruction(popcnt)
		value := popcnt.Return()
		state.push(value)

	case wasm.OpcodeI32WrapI64:
		if state.unreachable {
			break
		}
		x := state.pop()
		wrap := builder.AllocateInstruction().AsIreduce(x, ssa.TypeI32).Insert(builder).Return()
		state.push(wrap)
	case wasm.OpcodeGlobalGet:
		index := c.readI32u()
		if state.unreachable {
			break
		}
		v := c.getWasmGlobalValue(index, false)
		state.push(v)
	case wasm.OpcodeGlobalSet:
		index := c.readI32u()
		if state.unreachable {
			break
		}
		v := state.pop()
		c.setWasmGlobalValue(index, v)
	case wasm.OpcodeLocalGet:
		index := c.readI32u()
		if state.unreachable {
			break
		}
		variable := c.localVariable(index)
		state.push(builder.MustFindValue(variable))

	case wasm.OpcodeLocalSet:
		index := c.readI32u()
		if state.unreachable {
			break
		}
		variable := c.localVariable(index)
		newValue := state.pop()
		builder.DefineVariableInCurrentBB(variable, newValue)

	case wasm.OpcodeLocalTee:
		index := c.readI32u()
		if state.unreachable {
			break
		}
		variable := c.localVariable(index)
		newValue := state.peek()
		builder.DefineVariableInCurrentBB(variable, newValue)

	case wasm.OpcodeSelect, wasm.OpcodeTypedSelect:
		if op == wasm.OpcodeTypedSelect {
			state.pc += 2 // ignores the type which is only needed during validation.
		}

		if state.unreachable {
			break
		}

		cond := state.pop()
		v2 := state.pop()
		v1 := state.pop()

		sl := builder.AllocateInstruction().
			AsSelect(cond, v1, v2).
			Insert(builder).
			Return()
		state.push(sl)

	case wasm.OpcodeMemorySize:
		state.pc++ // skips the memory index.
		if state.unreachable {
			break
		}

		var memSizeInBytes ssa.Value
		if c.offset.LocalMemoryBegin < 0 {
			memInstPtr := builder.AllocateInstruction().
				AsLoad(c.moduleCtxPtrValue, c.offset.ImportedMemoryBegin.U32(), ssa.TypeI64).
				Insert(builder).
				Return()

			memSizeInBytes = builder.AllocateInstruction().
				AsLoad(memInstPtr, memoryInstanceBufSizeOffset, ssa.TypeI32).
				Insert(builder).
				Return()
		} else {
			memSizeInBytes = builder.AllocateInstruction().
				AsLoad(c.moduleCtxPtrValue, c.offset.LocalMemoryLen().U32(), ssa.TypeI32).
				Insert(builder).
				Return()
		}

		amount := builder.AllocateInstruction()
		amount.AsIconst32(uint32(wasm.MemoryPageSizeInBits))
		builder.InsertInstruction(amount)
		memSize := builder.AllocateInstruction().
			AsUshr(memSizeInBytes, amount.Return()).
			Insert(builder).
			Return()
		state.push(memSize)

	case wasm.OpcodeMemoryGrow:
		state.pc++ // skips the memory index.
		if state.unreachable {
			break
		}

		c.storeCallerModuleContext()

		pages := state.pop()
		memoryGrowPtr := builder.AllocateInstruction().
			AsLoad(c.execCtxPtrValue,
				wazevoapi.ExecutionContextOffsetMemoryGrowTrampolineAddress.U32(),
				ssa.TypeI64,
			).Insert(builder).Return()

		args := c.allocateVarLengthValues(1, c.execCtxPtrValue, pages)
		callGrowRet := builder.
			AllocateInstruction().
			AsCallIndirect(memoryGrowPtr, &c.memoryGrowSig, args).
			Insert(builder).Return()
		state.push(callGrowRet)

		// After the memory grow, reload the cached memory base and len.
		c.reloadMemoryBaseLen()

	case wasm.OpcodeI32Store,
		wasm.OpcodeI64Store,
		wasm.OpcodeF32Store,
		wasm.OpcodeF64Store,
		wasm.OpcodeI32Store8,
		wasm.OpcodeI32Store16,
		wasm.OpcodeI64Store8,
		wasm.OpcodeI64Store16,
		wasm.OpcodeI64Store32:

		_, offset := c.readMemArg()
		if state.unreachable {
			break
		}
		var opSize uint64
		var opcode ssa.Opcode
		switch op {
		case wasm.OpcodeI32Store, wasm.OpcodeF32Store:
			opcode = ssa.OpcodeStore
			opSize = 4
		case wasm.OpcodeI64Store, wasm.OpcodeF64Store:
			opcode = ssa.OpcodeStore
			opSize = 8
		case wasm.OpcodeI32Store8, wasm.OpcodeI64Store8:
			opcode = ssa.OpcodeIstore8
			opSize = 1
		case wasm.OpcodeI32Store16, wasm.OpcodeI64Store16:
			opcode = ssa.OpcodeIstore16
			opSize = 2
		case wasm.OpcodeI64Store32:
			opcode = ssa.OpcodeIstore32
			opSize = 4
		default:
			panic("BUG")
		}

		value := state.pop()
		baseAddr := state.pop()
		addr := c.memOpSetup(baseAddr, uint64(offset), opSize)
		builder.AllocateInstruction().
			AsStore(opcode, value, addr, offset).
			Insert(builder)

	case wasm.OpcodeI32Load,
		wasm.OpcodeI64Load,
		wasm.OpcodeF32Load,
		wasm.OpcodeF64Load,
		wasm.OpcodeI32Load8S,
		wasm.OpcodeI32Load8U,
		wasm.OpcodeI32Load16S,
		wasm.OpcodeI32Load16U,
		wasm.OpcodeI64Load8S,
		wasm.OpcodeI64Load8U,
		wasm.OpcodeI64Load16S,
		wasm.OpcodeI64Load16U,
		wasm.OpcodeI64Load32S,
		wasm.OpcodeI64Load32U:
		_, offset := c.readMemArg()
		if state.unreachable {
			break
		}

		var opSize uint64
		switch op {
		case wasm.OpcodeI32Load, wasm.OpcodeF32Load:
			opSize = 4
		case wasm.OpcodeI64Load, wasm.OpcodeF64Load:
			opSize = 8
		case wasm.OpcodeI32Load8S, wasm.OpcodeI32Load8U:
			opSize = 1
		case wasm.OpcodeI32Load16S, wasm.OpcodeI32Load16U:
			opSize = 2
		case wasm.OpcodeI64Load8S, wasm.OpcodeI64Load8U:
			opSize = 1
		case wasm.OpcodeI64Load16S, wasm.OpcodeI64Load16U:
			opSize = 2
		case wasm.OpcodeI64Load32S, wasm.OpcodeI64Load32U:
			opSize = 4
		default:
			panic("BUG")
		}

		baseAddr := state.pop()
		addr := c.memOpSetup(baseAddr, uint64(offset), opSize)
		load := builder.AllocateInstruction()
		switch op {
		case wasm.OpcodeI32Load:
			load.AsLoad(addr, offset, ssa.TypeI32)
		case wasm.OpcodeI64Load:
			load.AsLoad(addr, offset, ssa.TypeI64)
		case wasm.OpcodeF32Load:
			load.AsLoad(addr, offset, ssa.TypeF32)
		case wasm.OpcodeF64Load:
			load.AsLoad(addr, offset, ssa.TypeF64)
		case wasm.OpcodeI32Load8S:
			load.AsExtLoad(ssa.OpcodeSload8, addr, offset, false)
		case wasm.OpcodeI32Load8U:
			load.AsExtLoad(ssa.OpcodeUload8, addr, offset, false)
		case wasm.OpcodeI32Load16S:
			load.AsExtLoad(ssa.OpcodeSload16, addr, offset, false)
		case wasm.OpcodeI32Load16U:
			load.AsExtLoad(ssa.OpcodeUload16, addr, offset, false)
		case wasm.OpcodeI64Load8S:
			load.AsExtLoad(ssa.OpcodeSload8, addr, offset, true)
		case wasm.OpcodeI64Load8U:
			load.AsExtLoad(ssa.OpcodeUload8, addr, offset, true)
		case wasm.OpcodeI64Load16S:
			load.AsExtLoad(ssa.OpcodeSload16, addr, offset, true)
		case wasm.OpcodeI64Load16U:
			load.AsExtLoad(ssa.OpcodeUload16, addr, offset, true)
		case wasm.OpcodeI64Load32S:
			load.AsExtLoad(ssa.OpcodeSload32, addr, offset, true)
		case wasm.OpcodeI64Load32U:
			load.AsExtLoad(ssa.OpcodeUload32, addr, offset, true)
		default:
			panic("BUG")
		}
		builder.InsertInstruction(load)
		state.push(load.Return())
	case wasm.OpcodeBlock:
		// Note: we do not need to create a BB for this as that would always have only one predecessor
		// which is the current BB, and therefore it's always ok to merge them in any way.

		bt := c.readBlockType()

		if state.unreachable {
			state.unreachableDepth++
			break
		}

		followingBlk := builder.AllocateBasicBlock()
		c.addBlockParamsFromWasmTypes(bt.Results, followingBlk)

		state.ctrlPush(controlFrame{
			kind:                         controlFrameKindBlock,
			originalStackLenWithoutParam: len(state.values) - len(bt.Params),
			followingBlock:               followingBlk,
			blockType:                    bt,
		})
	case wasm.OpcodeLoop:
		bt := c.readBlockType()

		if state.unreachable {
			state.unreachableDepth++
			break
		}

		loopHeader, afterLoopBlock := builder.AllocateBasicBlock(), builder.AllocateBasicBlock()
		c.addBlockParamsFromWasmTypes(bt.Params, loopHeader)
		c.addBlockParamsFromWasmTypes(bt.Results, afterLoopBlock)

		originalLen := len(state.values) - len(bt.Params)
		state.ctrlPush(controlFrame{
			originalStackLenWithoutParam: originalLen,
			kind:                         controlFrameKindLoop,
			blk:                          loopHeader,
			followingBlock:               afterLoopBlock,
			blockType:                    bt,
		})

		args := c.allocateVarLengthValues(originalLen)
		args = args.Append(builder.VarLengthPool(), state.values[originalLen:]...)

		// Insert the jump to the header of loop.
		br := builder.AllocateInstruction()
		br.AsJump(args, loopHeader)
		builder.InsertInstruction(br)

		c.switchTo(originalLen, loopHeader)

		if c.ensureTermination {
			checkModuleExitCodePtr := builder.AllocateInstruction().
				AsLoad(c.execCtxPtrValue,
					wazevoapi.ExecutionContextOffsetCheckModuleExitCodeTrampolineAddress.U32(),
					ssa.TypeI64,
				).Insert(builder).Return()

			args := c.allocateVarLengthValues(1, c.execCtxPtrValue)
			builder.AllocateInstruction().
				AsCallIndirect(checkModuleExitCodePtr, &c.checkModuleExitCodeSig, args).
				Insert(builder)
		}
	case wasm.OpcodeIf:
		bt := c.readBlockType()

		if state.unreachable {
			state.unreachableDepth++
			break
		}

		v := state.pop()
		thenBlk, elseBlk, followingBlk := builder.AllocateBasicBlock(), builder.AllocateBasicBlock(), builder.AllocateBasicBlock()

		// We do not make the Wasm-level block parameters as SSA-level block params for if-else blocks
		// since they won't be PHI and the definition is unique.

		// On the other hand, the following block after if-else-end will likely have
		// multiple definitions (one in Then and another in Else blocks).
		c.addBlockParamsFromWasmTypes(bt.Results, followingBlk)

		args := c.allocateVarLengthValues(len(bt.Params))
		args = args.Append(builder.VarLengthPool(), state.values[len(state.values)-len(bt.Params):]...)

		// Insert the conditional jump to the Else block.
		brz := builder.AllocateInstruction()
		brz.AsBrz(v, ssa.ValuesNil, elseBlk)
		builder.InsertInstruction(brz)

		// Then, insert the jump to the Then block.
		br := builder.AllocateInstruction()
		br.AsJump(ssa.ValuesNil, thenBlk)
		builder.InsertInstruction(br)

		state.ctrlPush(controlFrame{
			kind:                         controlFrameKindIfWithoutElse,
			originalStackLenWithoutParam: len(state.values) - len(bt.Params),
			blk:                          elseBlk,
			followingBlock:               followingBlk,
			blockType:                    bt,
			clonedArgs:                   args,
		})

		builder.SetCurrentBlock(thenBlk)

		// Then and Else (if exists) have only one predecessor.
		builder.Seal(thenBlk)
		builder.Seal(elseBlk)
	case wasm.OpcodeElse:
		ifctrl := state.ctrlPeekAt(0)
		if unreachable := state.unreachable; unreachable && state.unreachableDepth > 0 {
			// If it is currently in unreachable and is a nested if,
			// we just remove the entire else block.
			break
		}

		ifctrl.kind = controlFrameKindIfWithElse
		if !state.unreachable {
			// If this Then block is currently reachable, we have to insert the branching to the following BB.
			followingBlk := ifctrl.followingBlock // == the BB after if-then-else.
			args := c.nPeekDup(len(ifctrl.blockType.Results))
			c.insertJumpToBlock(args, followingBlk)
		} else {
			state.unreachable = false
		}

		// Reset the stack so that we can correctly handle the else block.
		state.values = state.values[:ifctrl.originalStackLenWithoutParam]
		elseBlk := ifctrl.blk
		for _, arg := range ifctrl.clonedArgs.View() {
			state.push(arg)
		}

		builder.SetCurrentBlock(elseBlk)

	case wasm.OpcodeEnd:
		if state.unreachableDepth > 0 {
			state.unreachableDepth--
			break
		}

		ctrl := state.ctrlPop()
		followingBlk := ctrl.followingBlock

		unreachable := state.unreachable
		if !unreachable {
			// Top n-th args will be used as a result of the current control frame.
			args := c.nPeekDup(len(ctrl.blockType.Results))

			// Insert the unconditional branch to the target.
			c.insertJumpToBlock(args, followingBlk)
		} else { // recover from the unreachable state.
			state.unreachable = false
		}

		switch ctrl.kind {
		case controlFrameKindFunction:
			break // This is the very end of function.
		case controlFrameKindLoop:
			// Loop header block can be reached from any br/br_table contained in the loop,
			// so now that we've reached End of it, we can seal it.
			builder.Seal(ctrl.blk)
		case controlFrameKindIfWithoutElse:
			// If this is the end of Then block, we have to emit the empty Else block.
			elseBlk := ctrl.blk
			builder.SetCurrentBlock(elseBlk)
			c.insertJumpToBlock(ctrl.clonedArgs, followingBlk)
		}

		builder.Seal(followingBlk)

		// Ready to start translating the following block.
		c.switchTo(ctrl.originalStackLenWithoutParam, followingBlk)

	case wasm.OpcodeBr:
		labelIndex := c.readI32u()
		if state.unreachable {
			break
		}

		targetBlk, argNum := state.brTargetArgNumFor(labelIndex)
		args := c.nPeekDup(argNum)
		c.insertJumpToBlock(args, targetBlk)

		state.unreachable = true

	case wasm.OpcodeBrIf:
		labelIndex := c.readI32u()
		if state.unreachable {
			break
		}

		v := state.pop()

		targetBlk, argNum := state.brTargetArgNumFor(labelIndex)
		args := c.nPeekDup(argNum)
		var sealTargetBlk bool
		if c.needListener && targetBlk.ReturnBlock() { // In this case, we have to call the listener before returning.
			// Save the currently active block.
			current := builder.CurrentBlock()

			// Allocate the trampoline block to the return where we call the listener.
			targetBlk = builder.AllocateBasicBlock()
			builder.SetCurrentBlock(targetBlk)
			sealTargetBlk = true

			c.callListenerAfter()

			instr := builder.AllocateInstruction()
			instr.AsReturn(args)
			builder.InsertInstruction(instr)

			args = ssa.ValuesNil

			// Revert the current block.
			builder.SetCurrentBlock(current)
		}

		// Insert the conditional jump to the target block.
		brnz := builder.AllocateInstruction()
		brnz.AsBrnz(v, args, targetBlk)
		builder.InsertInstruction(brnz)

		if sealTargetBlk {
			builder.Seal(targetBlk)
		}

		// Insert the unconditional jump to the Else block which corresponds to after br_if.
		elseBlk := builder.AllocateBasicBlock()
		c.insertJumpToBlock(ssa.ValuesNil, elseBlk)

		// Now start translating the instructions after br_if.
		builder.Seal(elseBlk) // Else of br_if has the current block as the only one successor.
		builder.SetCurrentBlock(elseBlk)

	case wasm.OpcodeBrTable:
		labels := state.tmpForBrTable[:0]
		labelCount := c.readI32u()
		for i := 0; i < int(labelCount); i++ {
			labels = append(labels, c.readI32u())
		}
		labels = append(labels, c.readI32u()) // default label.
		if state.unreachable {
			break
		}

		index := state.pop()
		if labelCount == 0 { // If this br_table is empty, we can just emit the unconditional jump.
			targetBlk, argNum := state.brTargetArgNumFor(labels[0])
			args := c.nPeekDup(argNum)
			c.insertJumpToBlock(args, targetBlk)
		} else {
			c.lowerBrTable(labels, index)
		}
		state.tmpForBrTable = labels // reuse the temporary slice for next use.
		state.unreachable = true

	case wasm.OpcodeNop:
	case wasm.OpcodeReturn:
		if state.unreachable {
			break
		}
		if c.needListener {
			c.callListenerAfter()
		}

		results := c.nPeekDup(c.results())
		instr := builder.AllocateInstruction()

		instr.AsReturn(results)
		builder.InsertInstruction(instr)
		state.unreachable = true

	case wasm.OpcodeUnreachable:
		if state.unreachable {
			break
		}
		exit := builder.AllocateInstruction()
		exit.AsExitWithCode(c.execCtxPtrValue, wazevoapi.ExitCodeUnreachable)
		builder.InsertInstruction(exit)
		state.unreachable = true

	case wasm.OpcodeCallIndirect:
		typeIndex := c.readI32u()
		tableIndex := c.readI32u()
		if state.unreachable {
			break
		}
		c.lowerCallIndirect(typeIndex, tableIndex)

	case wasm.OpcodeCall:
		fnIndex := c.readI32u()
		if state.unreachable {
			break
		}

		var typIndex wasm.Index
		if fnIndex < c.m.ImportFunctionCount {
			// Before transfer the control to the callee, we have to store the current module's moduleContextPtr
			// into execContext.callerModuleContextPtr in case when the callee is a Go function.
			c.storeCallerModuleContext()
			var fi int
			for i := range c.m.ImportSection {
				imp := &c.m.ImportSection[i]
				if imp.Type == wasm.ExternTypeFunc {
					if fi == int(fnIndex) {
						typIndex = imp.DescFunc
						break
					}
					fi++
				}
			}
		} else {
			typIndex = c.m.FunctionSection[fnIndex-c.m.ImportFunctionCount]
		}
		typ := &c.m.TypeSection[typIndex]

		argN := len(typ.Params)
		tail := len(state.values) - argN
		vs := state.values[tail:]
		state.values = state.values[:tail]
		args := c.allocateVarLengthValues(2+len(vs), c.execCtxPtrValue)

		sig := c.signatures[typ]
		call := builder.AllocateInstruction()
		if fnIndex >= c.m.ImportFunctionCount {
			args = args.Append(builder.VarLengthPool(), c.moduleCtxPtrValue) // This case the callee module is itself.
			args = args.Append(builder.VarLengthPool(), vs...)
			call.AsCall(FunctionIndexToFuncRef(fnIndex), sig, args)
			builder.InsertInstruction(call)
		} else {
			// This case we have to read the address of the imported function from the module context.
			moduleCtx := c.moduleCtxPtrValue
			loadFuncPtr, loadModuleCtxPtr := builder.AllocateInstruction(), builder.AllocateInstruction()
			funcPtrOffset, moduleCtxPtrOffset, _ := c.offset.ImportedFunctionOffset(fnIndex)
			loadFuncPtr.AsLoad(moduleCtx, funcPtrOffset.U32(), ssa.TypeI64)
			loadModuleCtxPtr.AsLoad(moduleCtx, moduleCtxPtrOffset.U32(), ssa.TypeI64)
			builder.InsertInstruction(loadFuncPtr)
			builder.InsertInstruction(loadModuleCtxPtr)

			args = args.Append(builder.VarLengthPool(), loadModuleCtxPtr.Return())
			args = args.Append(builder.VarLengthPool(), vs...)
			call.AsCallIndirect(loadFuncPtr.Return(), sig, args)
			builder.InsertInstruction(call)
		}

		first, rest := call.Returns()
		if first.Valid() {
			state.push(first)
		}
		for _, v := range rest {
			state.push(v)
		}

		c.reloadAfterCall()

	case wasm.OpcodeDrop:
		if state.unreachable {
			break
		}
		_ = state.pop()
	case wasm.OpcodeF64ConvertI32S, wasm.OpcodeF64ConvertI64S, wasm.OpcodeF64ConvertI32U, wasm.OpcodeF64ConvertI64U:
		if state.unreachable {
			break
		}
		result := builder.AllocateInstruction().AsFcvtFromInt(
			state.pop(),
			op == wasm.OpcodeF64ConvertI32S || op == wasm.OpcodeF64ConvertI64S,
			true,
		).Insert(builder).Return()
		state.push(result)
	case wasm.OpcodeF32ConvertI32S, wasm.OpcodeF32ConvertI64S, wasm.OpcodeF32ConvertI32U, wasm.OpcodeF32ConvertI64U:
		if state.unreachable {
			break
		}
		result := builder.AllocateInstruction().AsFcvtFromInt(
			state.pop(),
			op == wasm.OpcodeF32ConvertI32S || op == wasm.OpcodeF32ConvertI64S,
			false,
		).Insert(builder).Return()
		state.push(result)
	case wasm.OpcodeF32DemoteF64:
		if state.unreachable {
			break
		}
		cvt := builder.AllocateInstruction()
		cvt.AsFdemote(state.pop())
		builder.InsertInstruction(cvt)
		state.push(cvt.Return())
	case wasm.OpcodeF64PromoteF32:
		if state.unreachable {
			break
		}
		cvt := builder.AllocateInstruction()
		cvt.AsFpromote(state.pop())
		builder.InsertInstruction(cvt)
		state.push(cvt.Return())

	case wasm.OpcodeVecPrefix:
		state.pc++
		vecOp := c.wasmFunctionBody[state.pc]
		switch vecOp {
		case wasm.OpcodeVecV128Const:
			state.pc++
			lo := binary.LittleEndian.Uint64(c.wasmFunctionBody[state.pc:])
			state.pc += 8
			hi := binary.LittleEndian.Uint64(c.wasmFunctionBody[state.pc:])
			state.pc += 7
			if state.unreachable {
				break
			}
			ret := builder.AllocateInstruction().AsVconst(lo, hi).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecV128Load:
			_, offset := c.readMemArg()
			if state.unreachable {
				break
			}
			baseAddr := state.pop()
			addr := c.memOpSetup(baseAddr, uint64(offset), 16)
			load := builder.AllocateInstruction()
			load.AsLoad(addr, offset, ssa.TypeV128)
			builder.InsertInstruction(load)
			state.push(load.Return())
		case wasm.OpcodeVecV128Load8Lane, wasm.OpcodeVecV128Load16Lane, wasm.OpcodeVecV128Load32Lane:
			_, offset := c.readMemArg()
			state.pc++
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			var loadOp ssa.Opcode
			var opSize uint64
			switch vecOp {
			case wasm.OpcodeVecV128Load8Lane:
				loadOp, lane, opSize = ssa.OpcodeUload8, ssa.VecLaneI8x16, 1
			case wasm.OpcodeVecV128Load16Lane:
				loadOp, lane, opSize = ssa.OpcodeUload16, ssa.VecLaneI16x8, 2
			case wasm.OpcodeVecV128Load32Lane:
				loadOp, lane, opSize = ssa.OpcodeUload32, ssa.VecLaneI32x4, 4
			}
			laneIndex := c.wasmFunctionBody[state.pc]
			vector := state.pop()
			baseAddr := state.pop()
			addr := c.memOpSetup(baseAddr, uint64(offset), opSize)
			load := builder.AllocateInstruction().
				AsExtLoad(loadOp, addr, offset, false).
				Insert(builder).Return()
			ret := builder.AllocateInstruction().
				AsInsertlane(vector, load, laneIndex, lane).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecV128Load64Lane:
			_, offset := c.readMemArg()
			state.pc++
			if state.unreachable {
				break
			}
			laneIndex := c.wasmFunctionBody[state.pc]
			vector := state.pop()
			baseAddr := state.pop()
			addr := c.memOpSetup(baseAddr, uint64(offset), 8)
			load := builder.AllocateInstruction().
				AsLoad(addr, offset, ssa.TypeI64).
				Insert(builder).Return()
			ret := builder.AllocateInstruction().
				AsInsertlane(vector, load, laneIndex, ssa.VecLaneI64x2).
				Insert(builder).Return()
			state.push(ret)

		case wasm.OpcodeVecV128Load32zero, wasm.OpcodeVecV128Load64zero:
			_, offset := c.readMemArg()
			if state.unreachable {
				break
			}

			var scalarType ssa.Type
			switch vecOp {
			case wasm.OpcodeVecV128Load32zero:
				scalarType = ssa.TypeF32
			case wasm.OpcodeVecV128Load64zero:
				scalarType = ssa.TypeF64
			}

			baseAddr := state.pop()
			addr := c.memOpSetup(baseAddr, uint64(offset), uint64(scalarType.Size()))

			ret := builder.AllocateInstruction().
				AsVZeroExtLoad(addr, offset, scalarType).
				Insert(builder).Return()
			state.push(ret)

		case wasm.OpcodeVecV128Load8x8u, wasm.OpcodeVecV128Load8x8s,
			wasm.OpcodeVecV128Load16x4u, wasm.OpcodeVecV128Load16x4s,
			wasm.OpcodeVecV128Load32x2u, wasm.OpcodeVecV128Load32x2s:
			_, offset := c.readMemArg()
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			var signed bool
			switch vecOp {
			case wasm.OpcodeVecV128Load8x8s:
				signed = true
				fallthrough
			case wasm.OpcodeVecV128Load8x8u:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecV128Load16x4s:
				signed = true
				fallthrough
			case wasm.OpcodeVecV128Load16x4u:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecV128Load32x2s:
				signed = true
				fallthrough
			case wasm.OpcodeVecV128Load32x2u:
				lane = ssa.VecLaneI32x4
			}
			baseAddr := state.pop()
			addr := c.memOpSetup(baseAddr, uint64(offset), 8)
			load := builder.AllocateInstruction().
				AsLoad(addr, offset, ssa.TypeF64).
				Insert(builder).Return()
			ret := builder.AllocateInstruction().
				AsWiden(load, lane, signed, true).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecV128Load8Splat, wasm.OpcodeVecV128Load16Splat,
			wasm.OpcodeVecV128Load32Splat, wasm.OpcodeVecV128Load64Splat:
			_, offset := c.readMemArg()
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			var opSize uint64
			switch vecOp {
			case wasm.OpcodeVecV128Load8Splat:
				lane, opSize = ssa.VecLaneI8x16, 1
			case wasm.OpcodeVecV128Load16Splat:
				lane, opSize = ssa.VecLaneI16x8, 2
			case wasm.OpcodeVecV128Load32Splat:
				lane, opSize = ssa.VecLaneI32x4, 4
			case wasm.OpcodeVecV128Load64Splat:
				lane, opSize = ssa.VecLaneI64x2, 8
			}
			baseAddr := state.pop()
			addr := c.memOpSetup(baseAddr, uint64(offset), opSize)
			ret := builder.AllocateInstruction().
				AsLoadSplat(addr, offset, lane).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecV128Store:
			_, offset := c.readMemArg()
			if state.unreachable {
				break
			}
			value := state.pop()
			baseAddr := state.pop()
			addr := c.memOpSetup(baseAddr, uint64(offset), 16)
			builder.AllocateInstruction().
				AsStore(ssa.OpcodeStore, value, addr, offset).
				Insert(builder)
		case wasm.OpcodeVecV128Store8Lane, wasm.OpcodeVecV128Store16Lane,
			wasm.OpcodeVecV128Store32Lane, wasm.OpcodeVecV128Store64Lane:
			_, offset := c.readMemArg()
			state.pc++
			if state.unreachable {
				break
			}
			laneIndex := c.wasmFunctionBody[state.pc]
			var storeOp ssa.Opcode
			var lane ssa.VecLane
			var opSize uint64
			switch vecOp {
			case wasm.OpcodeVecV128Store8Lane:
				storeOp, lane, opSize = ssa.OpcodeIstore8, ssa.VecLaneI8x16, 1
			case wasm.OpcodeVecV128Store16Lane:
				storeOp, lane, opSize = ssa.OpcodeIstore16, ssa.VecLaneI16x8, 2
			case wasm.OpcodeVecV128Store32Lane:
				storeOp, lane, opSize = ssa.OpcodeIstore32, ssa.VecLaneI32x4, 4
			case wasm.OpcodeVecV128Store64Lane:
				storeOp, lane, opSize = ssa.OpcodeStore, ssa.VecLaneI64x2, 8
			}
			vector := state.pop()
			baseAddr := state.pop()
			addr := c.memOpSetup(baseAddr, uint64(offset), opSize)
			value := builder.AllocateInstruction().
				AsExtractlane(vector, laneIndex, lane, false).
				Insert(builder).Return()
			builder.AllocateInstruction().
				AsStore(storeOp, value, addr, offset).
				Insert(builder)
		case wasm.OpcodeVecV128Not:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVbnot(v1).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecV128And:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVband(v1, v2).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecV128AndNot:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVbandnot(v1, v2).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecV128Or:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVbor(v1, v2).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecV128Xor:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVbxor(v1, v2).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecV128Bitselect:
			if state.unreachable {
				break
			}
			c := state.pop()
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVbitselect(c, v1, v2).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecV128AnyTrue:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVanyTrue(v1).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16AllTrue, wasm.OpcodeVecI16x8AllTrue, wasm.OpcodeVecI32x4AllTrue, wasm.OpcodeVecI64x2AllTrue:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16AllTrue:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8AllTrue:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4AllTrue:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2AllTrue:
				lane = ssa.VecLaneI64x2
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVallTrue(v1, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16BitMask, wasm.OpcodeVecI16x8BitMask, wasm.OpcodeVecI32x4BitMask, wasm.OpcodeVecI64x2BitMask:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16BitMask:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8BitMask:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4BitMask:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2BitMask:
				lane = ssa.VecLaneI64x2
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVhighBits(v1, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16Abs, wasm.OpcodeVecI16x8Abs, wasm.OpcodeVecI32x4Abs, wasm.OpcodeVecI64x2Abs:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16Abs:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8Abs:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4Abs:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2Abs:
				lane = ssa.VecLaneI64x2
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVIabs(v1, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16Neg, wasm.OpcodeVecI16x8Neg, wasm.OpcodeVecI32x4Neg, wasm.OpcodeVecI64x2Neg:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16Neg:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8Neg:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4Neg:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2Neg:
				lane = ssa.VecLaneI64x2
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVIneg(v1, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16Popcnt:
			if state.unreachable {
				break
			}
			lane := ssa.VecLaneI8x16
			v1 := state.pop()

			ret := builder.AllocateInstruction().AsVIpopcnt(v1, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16Add, wasm.OpcodeVecI16x8Add, wasm.OpcodeVecI32x4Add, wasm.OpcodeVecI64x2Add:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16Add:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8Add:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4Add:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2Add:
				lane = ssa.VecLaneI64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVIadd(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16AddSatS, wasm.OpcodeVecI16x8AddSatS:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16AddSatS:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8AddSatS:
				lane = ssa.VecLaneI16x8
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVSaddSat(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16AddSatU, wasm.OpcodeVecI16x8AddSatU:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16AddSatU:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8AddSatU:
				lane = ssa.VecLaneI16x8
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVUaddSat(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16SubSatS, wasm.OpcodeVecI16x8SubSatS:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16SubSatS:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8SubSatS:
				lane = ssa.VecLaneI16x8
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVSsubSat(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16SubSatU, wasm.OpcodeVecI16x8SubSatU:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16SubSatU:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8SubSatU:
				lane = ssa.VecLaneI16x8
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVUsubSat(v1, v2, lane).Insert(builder).Return()
			state.push(ret)

		case wasm.OpcodeVecI8x16Sub, wasm.OpcodeVecI16x8Sub, wasm.OpcodeVecI32x4Sub, wasm.OpcodeVecI64x2Sub:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16Sub:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8Sub:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4Sub:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2Sub:
				lane = ssa.VecLaneI64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVIsub(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16MinS, wasm.OpcodeVecI16x8MinS, wasm.OpcodeVecI32x4MinS:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16MinS:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8MinS:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4MinS:
				lane = ssa.VecLaneI32x4
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVImin(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16MinU, wasm.OpcodeVecI16x8MinU, wasm.OpcodeVecI32x4MinU:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16MinU:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8MinU:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4MinU:
				lane = ssa.VecLaneI32x4
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVUmin(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16MaxS, wasm.OpcodeVecI16x8MaxS, wasm.OpcodeVecI32x4MaxS:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16MaxS:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8MaxS:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4MaxS:
				lane = ssa.VecLaneI32x4
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVImax(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16MaxU, wasm.OpcodeVecI16x8MaxU, wasm.OpcodeVecI32x4MaxU:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16MaxU:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8MaxU:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4MaxU:
				lane = ssa.VecLaneI32x4
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVUmax(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16AvgrU, wasm.OpcodeVecI16x8AvgrU:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16AvgrU:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8AvgrU:
				lane = ssa.VecLaneI16x8
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVAvgRound(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI16x8Mul, wasm.OpcodeVecI32x4Mul, wasm.OpcodeVecI64x2Mul:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI16x8Mul:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4Mul:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2Mul:
				lane = ssa.VecLaneI64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVImul(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI16x8Q15mulrSatS:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsSqmulRoundSat(v1, v2, ssa.VecLaneI16x8).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16Eq, wasm.OpcodeVecI16x8Eq, wasm.OpcodeVecI32x4Eq, wasm.OpcodeVecI64x2Eq:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16Eq:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8Eq:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4Eq:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2Eq:
				lane = ssa.VecLaneI64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVIcmp(v1, v2, ssa.IntegerCmpCondEqual, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16Ne, wasm.OpcodeVecI16x8Ne, wasm.OpcodeVecI32x4Ne, wasm.OpcodeVecI64x2Ne:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16Ne:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8Ne:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4Ne:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2Ne:
				lane = ssa.VecLaneI64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVIcmp(v1, v2, ssa.IntegerCmpCondNotEqual, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16LtS, wasm.OpcodeVecI16x8LtS, wasm.OpcodeVecI32x4LtS, wasm.OpcodeVecI64x2LtS:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16LtS:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8LtS:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4LtS:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2LtS:
				lane = ssa.VecLaneI64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVIcmp(v1, v2, ssa.IntegerCmpCondSignedLessThan, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16LtU, wasm.OpcodeVecI16x8LtU, wasm.OpcodeVecI32x4LtU:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16LtU:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8LtU:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4LtU:
				lane = ssa.VecLaneI32x4
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVIcmp(v1, v2, ssa.IntegerCmpCondUnsignedLessThan, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16LeS, wasm.OpcodeVecI16x8LeS, wasm.OpcodeVecI32x4LeS, wasm.OpcodeVecI64x2LeS:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16LeS:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8LeS:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4LeS:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2LeS:
				lane = ssa.VecLaneI64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVIcmp(v1, v2, ssa.IntegerCmpCondSignedLessThanOrEqual, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16LeU, wasm.OpcodeVecI16x8LeU, wasm.OpcodeVecI32x4LeU:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16LeU:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8LeU:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4LeU:
				lane = ssa.VecLaneI32x4
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVIcmp(v1, v2, ssa.IntegerCmpCondUnsignedLessThanOrEqual, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16GtS, wasm.OpcodeVecI16x8GtS, wasm.OpcodeVecI32x4GtS, wasm.OpcodeVecI64x2GtS:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16GtS:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8GtS:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4GtS:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2GtS:
				lane = ssa.VecLaneI64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVIcmp(v1, v2, ssa.IntegerCmpCondSignedGreaterThan, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16GtU, wasm.OpcodeVecI16x8GtU, wasm.OpcodeVecI32x4GtU:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16GtU:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8GtU:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4GtU:
				lane = ssa.VecLaneI32x4
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVIcmp(v1, v2, ssa.IntegerCmpCondUnsignedGreaterThan, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16GeS, wasm.OpcodeVecI16x8GeS, wasm.OpcodeVecI32x4GeS, wasm.OpcodeVecI64x2GeS:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16GeS:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8GeS:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4GeS:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2GeS:
				lane = ssa.VecLaneI64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVIcmp(v1, v2, ssa.IntegerCmpCondSignedGreaterThanOrEqual, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16GeU, wasm.OpcodeVecI16x8GeU, wasm.OpcodeVecI32x4GeU:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16GeU:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8GeU:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4GeU:
				lane = ssa.VecLaneI32x4
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVIcmp(v1, v2, ssa.IntegerCmpCondUnsignedGreaterThanOrEqual, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Max, wasm.OpcodeVecF64x2Max:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Max:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Max:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVFmax(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Abs, wasm.OpcodeVecF64x2Abs:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Abs:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Abs:
				lane = ssa.VecLaneF64x2
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVFabs(v1, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Min, wasm.OpcodeVecF64x2Min:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Min:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Min:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVFmin(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Neg, wasm.OpcodeVecF64x2Neg:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Neg:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Neg:
				lane = ssa.VecLaneF64x2
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVFneg(v1, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Sqrt, wasm.OpcodeVecF64x2Sqrt:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Sqrt:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Sqrt:
				lane = ssa.VecLaneF64x2
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVSqrt(v1, lane).Insert(builder).Return()
			state.push(ret)

		case wasm.OpcodeVecF32x4Add, wasm.OpcodeVecF64x2Add:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Add:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Add:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVFadd(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Sub, wasm.OpcodeVecF64x2Sub:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Sub:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Sub:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVFsub(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Mul, wasm.OpcodeVecF64x2Mul:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Mul:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Mul:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVFmul(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Div, wasm.OpcodeVecF64x2Div:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Div:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Div:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVFdiv(v1, v2, lane).Insert(builder).Return()
			state.push(ret)

		case wasm.OpcodeVecI16x8ExtaddPairwiseI8x16S, wasm.OpcodeVecI16x8ExtaddPairwiseI8x16U:
			if state.unreachable {
				break
			}
			v := state.pop()
			signed := vecOp == wasm.OpcodeVecI16x8ExtaddPairwiseI8x16S
			ret := builder.AllocateInstruction().AsExtIaddPairwise(v, ssa.VecLaneI8x16, signed).Insert(builder).Return()
			state.push(ret)

		case wasm.OpcodeVecI32x4ExtaddPairwiseI16x8S, wasm.OpcodeVecI32x4ExtaddPairwiseI16x8U:
			if state.unreachable {
				break
			}
			v := state.pop()
			signed := vecOp == wasm.OpcodeVecI32x4ExtaddPairwiseI16x8S
			ret := builder.AllocateInstruction().AsExtIaddPairwise(v, ssa.VecLaneI16x8, signed).Insert(builder).Return()
			state.push(ret)

		case wasm.OpcodeVecI16x8ExtMulLowI8x16S, wasm.OpcodeVecI16x8ExtMulLowI8x16U:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := c.lowerExtMul(
				v1, v2,
				ssa.VecLaneI8x16, ssa.VecLaneI16x8,
				vecOp == wasm.OpcodeVecI16x8ExtMulLowI8x16S, true)
			state.push(ret)

		case wasm.OpcodeVecI16x8ExtMulHighI8x16S, wasm.OpcodeVecI16x8ExtMulHighI8x16U:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := c.lowerExtMul(
				v1, v2,
				ssa.VecLaneI8x16, ssa.VecLaneI16x8,
				vecOp == wasm.OpcodeVecI16x8ExtMulHighI8x16S, false)
			state.push(ret)

		case wasm.OpcodeVecI32x4ExtMulLowI16x8S, wasm.OpcodeVecI32x4ExtMulLowI16x8U:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := c.lowerExtMul(
				v1, v2,
				ssa.VecLaneI16x8, ssa.VecLaneI32x4,
				vecOp == wasm.OpcodeVecI32x4ExtMulLowI16x8S, true)
			state.push(ret)

		case wasm.OpcodeVecI32x4ExtMulHighI16x8S, wasm.OpcodeVecI32x4ExtMulHighI16x8U:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := c.lowerExtMul(
				v1, v2,
				ssa.VecLaneI16x8, ssa.VecLaneI32x4,
				vecOp == wasm.OpcodeVecI32x4ExtMulHighI16x8S, false)
			state.push(ret)
		case wasm.OpcodeVecI64x2ExtMulLowI32x4S, wasm.OpcodeVecI64x2ExtMulLowI32x4U:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := c.lowerExtMul(
				v1, v2,
				ssa.VecLaneI32x4, ssa.VecLaneI64x2,
				vecOp == wasm.OpcodeVecI64x2ExtMulLowI32x4S, true)
			state.push(ret)

		case wasm.OpcodeVecI64x2ExtMulHighI32x4S, wasm.OpcodeVecI64x2ExtMulHighI32x4U:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := c.lowerExtMul(
				v1, v2,
				ssa.VecLaneI32x4, ssa.VecLaneI64x2,
				vecOp == wasm.OpcodeVecI64x2ExtMulHighI32x4S, false)
			state.push(ret)

		case wasm.OpcodeVecI32x4DotI16x8S:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()

			ret := builder.AllocateInstruction().AsWideningPairwiseDotProductS(v1, v2).Insert(builder).Return()
			state.push(ret)

		case wasm.OpcodeVecF32x4Eq, wasm.OpcodeVecF64x2Eq:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Eq:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Eq:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVFcmp(v1, v2, ssa.FloatCmpCondEqual, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Ne, wasm.OpcodeVecF64x2Ne:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Ne:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Ne:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVFcmp(v1, v2, ssa.FloatCmpCondNotEqual, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Lt, wasm.OpcodeVecF64x2Lt:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Lt:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Lt:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVFcmp(v1, v2, ssa.FloatCmpCondLessThan, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Le, wasm.OpcodeVecF64x2Le:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Le:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Le:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVFcmp(v1, v2, ssa.FloatCmpCondLessThanOrEqual, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Gt, wasm.OpcodeVecF64x2Gt:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Gt:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Gt:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVFcmp(v1, v2, ssa.FloatCmpCondGreaterThan, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Ge, wasm.OpcodeVecF64x2Ge:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Ge:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Ge:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVFcmp(v1, v2, ssa.FloatCmpCondGreaterThanOrEqual, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Ceil, wasm.OpcodeVecF64x2Ceil:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Ceil:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Ceil:
				lane = ssa.VecLaneF64x2
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVCeil(v1, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Floor, wasm.OpcodeVecF64x2Floor:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Floor:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Floor:
				lane = ssa.VecLaneF64x2
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVFloor(v1, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Trunc, wasm.OpcodeVecF64x2Trunc:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Trunc:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Trunc:
				lane = ssa.VecLaneF64x2
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVTrunc(v1, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Nearest, wasm.OpcodeVecF64x2Nearest:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Nearest:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Nearest:
				lane = ssa.VecLaneF64x2
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVNearest(v1, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Pmin, wasm.OpcodeVecF64x2Pmin:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Pmin:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Pmin:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVMinPseudo(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4Pmax, wasm.OpcodeVecF64x2Pmax:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecF32x4Pmax:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Pmax:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVMaxPseudo(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI32x4TruncSatF32x4S, wasm.OpcodeVecI32x4TruncSatF32x4U:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVFcvtToIntSat(v1, ssa.VecLaneF32x4, vecOp == wasm.OpcodeVecI32x4TruncSatF32x4S).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI32x4TruncSatF64x2SZero, wasm.OpcodeVecI32x4TruncSatF64x2UZero:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVFcvtToIntSat(v1, ssa.VecLaneF64x2, vecOp == wasm.OpcodeVecI32x4TruncSatF64x2SZero).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4ConvertI32x4S, wasm.OpcodeVecF32x4ConvertI32x4U:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsVFcvtFromInt(v1, ssa.VecLaneF32x4, vecOp == wasm.OpcodeVecF32x4ConvertI32x4S).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF64x2ConvertLowI32x4S, wasm.OpcodeVecF64x2ConvertLowI32x4U:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			if runtime.GOARCH == "arm64" {
				// TODO: this is weird. fix.
				v1 = builder.AllocateInstruction().
					AsWiden(v1, ssa.VecLaneI32x4, vecOp == wasm.OpcodeVecF64x2ConvertLowI32x4S, true).Insert(builder).Return()
			}
			ret := builder.AllocateInstruction().
				AsVFcvtFromInt(v1, ssa.VecLaneF64x2, vecOp == wasm.OpcodeVecF64x2ConvertLowI32x4S).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16NarrowI16x8S, wasm.OpcodeVecI8x16NarrowI16x8U:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsNarrow(v1, v2, ssa.VecLaneI16x8, vecOp == wasm.OpcodeVecI8x16NarrowI16x8S).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI16x8NarrowI32x4S, wasm.OpcodeVecI16x8NarrowI32x4U:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsNarrow(v1, v2, ssa.VecLaneI32x4, vecOp == wasm.OpcodeVecI16x8NarrowI32x4S).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI16x8ExtendLowI8x16S, wasm.OpcodeVecI16x8ExtendLowI8x16U:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsWiden(v1, ssa.VecLaneI8x16, vecOp == wasm.OpcodeVecI16x8ExtendLowI8x16S, true).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI16x8ExtendHighI8x16S, wasm.OpcodeVecI16x8ExtendHighI8x16U:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsWiden(v1, ssa.VecLaneI8x16, vecOp == wasm.OpcodeVecI16x8ExtendHighI8x16S, false).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI32x4ExtendLowI16x8S, wasm.OpcodeVecI32x4ExtendLowI16x8U:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsWiden(v1, ssa.VecLaneI16x8, vecOp == wasm.OpcodeVecI32x4ExtendLowI16x8S, true).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI32x4ExtendHighI16x8S, wasm.OpcodeVecI32x4ExtendHighI16x8U:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsWiden(v1, ssa.VecLaneI16x8, vecOp == wasm.OpcodeVecI32x4ExtendHighI16x8S, false).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI64x2ExtendLowI32x4S, wasm.OpcodeVecI64x2ExtendLowI32x4U:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsWiden(v1, ssa.VecLaneI32x4, vecOp == wasm.OpcodeVecI64x2ExtendLowI32x4S, true).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI64x2ExtendHighI32x4S, wasm.OpcodeVecI64x2ExtendHighI32x4U:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsWiden(v1, ssa.VecLaneI32x4, vecOp == wasm.OpcodeVecI64x2ExtendHighI32x4S, false).
				Insert(builder).Return()
			state.push(ret)

		case wasm.OpcodeVecF64x2PromoteLowF32x4Zero:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsFvpromoteLow(v1, ssa.VecLaneF32x4).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecF32x4DemoteF64x2Zero:
			if state.unreachable {
				break
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().
				AsFvdemote(v1, ssa.VecLaneF64x2).
				Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16Shl, wasm.OpcodeVecI16x8Shl, wasm.OpcodeVecI32x4Shl, wasm.OpcodeVecI64x2Shl:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16Shl:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8Shl:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4Shl:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2Shl:
				lane = ssa.VecLaneI64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVIshl(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16ShrS, wasm.OpcodeVecI16x8ShrS, wasm.OpcodeVecI32x4ShrS, wasm.OpcodeVecI64x2ShrS:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16ShrS:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8ShrS:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4ShrS:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2ShrS:
				lane = ssa.VecLaneI64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVSshr(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16ShrU, wasm.OpcodeVecI16x8ShrU, wasm.OpcodeVecI32x4ShrU, wasm.OpcodeVecI64x2ShrU:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16ShrU:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8ShrU:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4ShrU:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2ShrU:
				lane = ssa.VecLaneI64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsVUshr(v1, v2, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecI8x16ExtractLaneS, wasm.OpcodeVecI16x8ExtractLaneS:
			state.pc++
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16ExtractLaneS:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8ExtractLaneS:
				lane = ssa.VecLaneI16x8
			}
			v1 := state.pop()
			index := c.wasmFunctionBody[state.pc]
			ext := builder.AllocateInstruction().AsExtractlane(v1, index, lane, true).Insert(builder).Return()
			state.push(ext)
		case wasm.OpcodeVecI8x16ExtractLaneU, wasm.OpcodeVecI16x8ExtractLaneU,
			wasm.OpcodeVecI32x4ExtractLane, wasm.OpcodeVecI64x2ExtractLane,
			wasm.OpcodeVecF32x4ExtractLane, wasm.OpcodeVecF64x2ExtractLane:
			state.pc++ // Skip the immediate value.
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16ExtractLaneU:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8ExtractLaneU:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4ExtractLane:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2ExtractLane:
				lane = ssa.VecLaneI64x2
			case wasm.OpcodeVecF32x4ExtractLane:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2ExtractLane:
				lane = ssa.VecLaneF64x2
			}
			v1 := state.pop()
			index := c.wasmFunctionBody[state.pc]
			ext := builder.AllocateInstruction().AsExtractlane(v1, index, lane, false).Insert(builder).Return()
			state.push(ext)
		case wasm.OpcodeVecI8x16ReplaceLane, wasm.OpcodeVecI16x8ReplaceLane,
			wasm.OpcodeVecI32x4ReplaceLane, wasm.OpcodeVecI64x2ReplaceLane,
			wasm.OpcodeVecF32x4ReplaceLane, wasm.OpcodeVecF64x2ReplaceLane:
			state.pc++
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16ReplaceLane:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8ReplaceLane:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4ReplaceLane:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2ReplaceLane:
				lane = ssa.VecLaneI64x2
			case wasm.OpcodeVecF32x4ReplaceLane:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2ReplaceLane:
				lane = ssa.VecLaneF64x2
			}
			v2 := state.pop()
			v1 := state.pop()
			index := c.wasmFunctionBody[state.pc]
			ret := builder.AllocateInstruction().AsInsertlane(v1, v2, index, lane).Insert(builder).Return()
			state.push(ret)
		case wasm.OpcodeVecV128i8x16Shuffle:
			state.pc++
			laneIndexes := c.wasmFunctionBody[state.pc : state.pc+16]
			state.pc += 15
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsShuffle(v1, v2, laneIndexes).Insert(builder).Return()
			state.push(ret)

		case wasm.OpcodeVecI8x16Swizzle:
			if state.unreachable {
				break
			}
			v2 := state.pop()
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsSwizzle(v1, v2, ssa.VecLaneI8x16).Insert(builder).Return()
			state.push(ret)

		case wasm.OpcodeVecI8x16Splat,
			wasm.OpcodeVecI16x8Splat,
			wasm.OpcodeVecI32x4Splat,
			wasm.OpcodeVecI64x2Splat,
			wasm.OpcodeVecF32x4Splat,
			wasm.OpcodeVecF64x2Splat:
			if state.unreachable {
				break
			}
			var lane ssa.VecLane
			switch vecOp {
			case wasm.OpcodeVecI8x16Splat:
				lane = ssa.VecLaneI8x16
			case wasm.OpcodeVecI16x8Splat:
				lane = ssa.VecLaneI16x8
			case wasm.OpcodeVecI32x4Splat:
				lane = ssa.VecLaneI32x4
			case wasm.OpcodeVecI64x2Splat:
				lane = ssa.VecLaneI64x2
			case wasm.OpcodeVecF32x4Splat:
				lane = ssa.VecLaneF32x4
			case wasm.OpcodeVecF64x2Splat:
				lane = ssa.VecLaneF64x2
			}
			v1 := state.pop()
			ret := builder.AllocateInstruction().AsSplat(v1, lane).Insert(builder).Return()
			state.push(ret)

		default:
			panic("TODO: unsupported vector instruction: " + wasm.VectorInstructionName(vecOp))
		}
	case wasm.OpcodeAtomicPrefix:
		state.pc++
		atomicOp := c.wasmFunctionBody[state.pc]
		switch atomicOp {
		case wasm.OpcodeAtomicMemoryWait32, wasm.OpcodeAtomicMemoryWait64:
			_, offset := c.readMemArg()
			if state.unreachable {
				break
			}

			c.storeCallerModuleContext()

			var opSize uint64
			var trampoline wazevoapi.Offset
			var sig *ssa.Signature
			switch atomicOp {
			case wasm.OpcodeAtomicMemoryWait32:
				opSize = 4
				trampoline = wazevoapi.ExecutionContextOffsetMemoryWait32TrampolineAddress
				sig = &c.memoryWait32Sig
			case wasm.OpcodeAtomicMemoryWait64:
				opSize = 8
				trampoline = wazevoapi.ExecutionContextOffsetMemoryWait64TrampolineAddress
				sig = &c.memoryWait64Sig
			}

			timeout := state.pop()
			exp := state.pop()
			baseAddr := state.pop()
			addr := c.atomicMemOpSetup(baseAddr, uint64(offset), opSize)

			memoryWaitPtr := builder.AllocateInstruction().
				AsLoad(c.execCtxPtrValue,
					trampoline.U32(),
					ssa.TypeI64,
				).Insert(builder).Return()

			args := c.allocateVarLengthValues(3, c.execCtxPtrValue, timeout, exp, addr)
			memoryWaitRet := builder.AllocateInstruction().
				AsCallIndirect(memoryWaitPtr, sig, args).
				Insert(builder).Return()
			state.push(memoryWaitRet)
		case wasm.OpcodeAtomicMemoryNotify:
			_, offset := c.readMemArg()
			if state.unreachable {
				break
			}

			c.storeCallerModuleContext()
			count := state.pop()
			baseAddr := state.pop()
			addr := c.atomicMemOpSetup(baseAddr, uint64(offset), 4)

			memoryNotifyPtr := builder.AllocateInstruction().
				AsLoad(c.execCtxPtrValue,
					wazevoapi.ExecutionContextOffsetMemoryNotifyTrampolineAddress.U32(),
					ssa.TypeI64,
				).Insert(builder).Return()
			args := c.allocateVarLengthValues(2, c.execCtxPtrValue, count, addr)
			memoryNotifyRet := builder.AllocateInstruction().
				AsCallIndirect(memoryNotifyPtr, &c.memoryNotifySig, args).
				Insert(builder).Return()
			state.push(memoryNotifyRet)
		case wasm.OpcodeAtomicI32Load, wasm.OpcodeAtomicI64Load, wasm.OpcodeAtomicI32Load8U, wasm.OpcodeAtomicI32Load16U, wasm.OpcodeAtomicI64Load8U, wasm.OpcodeAtomicI64Load16U, wasm.OpcodeAtomicI64Load32U:
			_, offset := c.readMemArg()
			if state.unreachable {
				break
			}

			baseAddr := state.pop()

			var size uint64
			switch atomicOp {
			case wasm.OpcodeAtomicI64Load:
				size = 8
			case wasm.OpcodeAtomicI32Load, wasm.OpcodeAtomicI64Load32U:
				size = 4
			case wasm.OpcodeAtomicI32Load16U, wasm.OpcodeAtomicI64Load16U:
				size = 2
			case wasm.OpcodeAtomicI32Load8U, wasm.OpcodeAtomicI64Load8U:
				size = 1
			}

			var typ ssa.Type
			switch atomicOp {
			case wasm.OpcodeAtomicI64Load, wasm.OpcodeAtomicI64Load32U, wasm.OpcodeAtomicI64Load16U, wasm.OpcodeAtomicI64Load8U:
				typ = ssa.TypeI64
			case wasm.OpcodeAtomicI32Load, wasm.OpcodeAtomicI32Load16U, wasm.OpcodeAtomicI32Load8U:
				typ = ssa.TypeI32
			}

			addr := c.atomicMemOpSetup(baseAddr, uint64(offset), size)
			res := builder.AllocateInstruction().AsAtomicLoad(addr, size, typ).Insert(builder).Return()
			state.push(res)
		case wasm.OpcodeAtomicI32Store, wasm.OpcodeAtomicI64Store, wasm.OpcodeAtomicI32Store8, wasm.OpcodeAtomicI32Store16, wasm.OpcodeAtomicI64Store8, wasm.OpcodeAtomicI64Store16, wasm.OpcodeAtomicI64Store32:
			_, offset := c.readMemArg()
			if state.unreachable {
				break
			}

			val := state.pop()
			baseAddr := state.pop()

			var size uint64
			switch atomicOp {
			case wasm.OpcodeAtomicI64Store:
				size = 8
			case wasm.OpcodeAtomicI32Store, wasm.OpcodeAtomicI64Store32:
				size = 4
			case wasm.OpcodeAtomicI32Store16, wasm.OpcodeAtomicI64Store16:
				size = 2
			case wasm.OpcodeAtomicI32Store8, wasm.OpcodeAtomicI64Store8:
				size = 1
			}

			addr := c.atomicMemOpSetup(baseAddr, uint64(offset), size)
			builder.AllocateInstruction().AsAtomicStore(addr, val, size).Insert(builder)
		case wasm.OpcodeAtomicI32RmwAdd, wasm.OpcodeAtomicI64RmwAdd, wasm.OpcodeAtomicI32Rmw8AddU, wasm.OpcodeAtomicI32Rmw16AddU, wasm.OpcodeAtomicI64Rmw8AddU, wasm.OpcodeAtomicI64Rmw16AddU, wasm.OpcodeAtomicI64Rmw32AddU,
			wasm.OpcodeAtomicI32RmwSub, wasm.OpcodeAtomicI64RmwSub, wasm.OpcodeAtomicI32Rmw8SubU, wasm.OpcodeAtomicI32Rmw16SubU, wasm.OpcodeAtomicI64Rmw8SubU, wasm.OpcodeAtomicI64Rmw16SubU, wasm.OpcodeAtomicI64Rmw32SubU,
			wasm.OpcodeAtomicI32RmwAnd, wasm.OpcodeAtomicI64RmwAnd, wasm.OpcodeAtomicI32Rmw8AndU, wasm.OpcodeAtomicI32Rmw16AndU, wasm.OpcodeAtomicI64Rmw8AndU, wasm.OpcodeAtomicI64Rmw16AndU, wasm.OpcodeAtomicI64Rmw32AndU,
			wasm.OpcodeAtomicI32RmwOr, wasm.OpcodeAtomicI64RmwOr, wasm.OpcodeAtomicI32Rmw8OrU, wasm.OpcodeAtomicI32Rmw16OrU, wasm.OpcodeAtomicI64Rmw8OrU, wasm.OpcodeAtomicI64Rmw16OrU, wasm.OpcodeAtomicI64Rmw32OrU,
			wasm.OpcodeAtomicI32RmwXor, wasm.OpcodeAtomicI64RmwXor, wasm.OpcodeAtomicI32Rmw8XorU, wasm.OpcodeAtomicI32Rmw16XorU, wasm.OpcodeAtomicI64Rmw8XorU, wasm.OpcodeAtomicI64Rmw16XorU, wasm.OpcodeAtomicI64Rmw32XorU,
			wasm.OpcodeAtomicI32RmwXchg, wasm.OpcodeAtomicI64RmwXchg, wasm.OpcodeAtomicI32Rmw8XchgU, wasm.OpcodeAtomicI32Rmw16XchgU, wasm.OpcodeAtomicI64Rmw8XchgU, wasm.OpcodeAtomicI64Rmw16XchgU, wasm.OpcodeAtomicI64Rmw32XchgU:
			_, offset := c.readMemArg()
			if state.unreachable {
				break
			}

			val := state.pop()
			baseAddr := state.pop()

			var rmwOp ssa.AtomicRmwOp
			var size uint64
			switch atomicOp {
			case wasm.OpcodeAtomicI32RmwAdd, wasm.OpcodeAtomicI64RmwAdd, wasm.OpcodeAtomicI32Rmw8AddU, wasm.OpcodeAtomicI32Rmw16AddU, wasm.OpcodeAtomicI64Rmw8AddU, wasm.OpcodeAtomicI64Rmw16AddU, wasm.OpcodeAtomicI64Rmw32AddU:
				rmwOp = ssa.AtomicRmwOpAdd
				switch atomicOp {
				case wasm.OpcodeAtomicI64RmwAdd:
					size = 8
				case wasm.OpcodeAtomicI32RmwAdd, wasm.OpcodeAtomicI64Rmw32AddU:
					size = 4
				case wasm.OpcodeAtomicI32Rmw16AddU, wasm.OpcodeAtomicI64Rmw16AddU:
					size = 2
				case wasm.OpcodeAtomicI32Rmw8AddU, wasm.OpcodeAtomicI64Rmw8AddU:
					size = 1
				}
			case wasm.OpcodeAtomicI32RmwSub, wasm.OpcodeAtomicI64RmwSub, wasm.OpcodeAtomicI32Rmw8SubU, wasm.OpcodeAtomicI32Rmw16SubU, wasm.OpcodeAtomicI64Rmw8SubU, wasm.OpcodeAtomicI64Rmw16SubU, wasm.OpcodeAtomicI64Rmw32SubU:
				rmwOp = ssa.AtomicRmwOpSub
				switch atomicOp {
				case wasm.OpcodeAtomicI64RmwSub:
					size = 8
				case wasm.OpcodeAtomicI32RmwSub, wasm.OpcodeAtomicI64Rmw32SubU:
					size = 4
				case wasm.OpcodeAtomicI32Rmw16SubU, wasm.OpcodeAtomicI64Rmw16SubU:
					size = 2
				case wasm.OpcodeAtomicI32Rmw8SubU, wasm.OpcodeAtomicI64Rmw8SubU:
					size = 1
				}
			case wasm.OpcodeAtomicI32RmwAnd, wasm.OpcodeAtomicI64RmwAnd, wasm.OpcodeAtomicI32Rmw8AndU, wasm.OpcodeAtomicI32Rmw16AndU, wasm.OpcodeAtomicI64Rmw8AndU, wasm.OpcodeAtomicI64Rmw16AndU, wasm.OpcodeAtomicI64Rmw32AndU:
				rmwOp = ssa.AtomicRmwOpAnd
				switch atomicOp {
				case wasm.OpcodeAtomicI64RmwAnd:
					size = 8
				case wasm.OpcodeAtomicI32RmwAnd, wasm.OpcodeAtomicI64Rmw32AndU:
					size = 4
				case wasm.OpcodeAtomicI32Rmw16AndU, wasm.OpcodeAtomicI64Rmw16AndU:
					size = 2
				case wasm.OpcodeAtomicI32Rmw8AndU, wasm.OpcodeAtomicI64Rmw8AndU:
					size = 1
				}
			case wasm.OpcodeAtomicI32RmwOr, wasm.OpcodeAtomicI64RmwOr, wasm.OpcodeAtomicI32Rmw8OrU, wasm.OpcodeAtomicI32Rmw16OrU, wasm.OpcodeAtomicI64Rmw8OrU, wasm.OpcodeAtomicI64Rmw16OrU, wasm.OpcodeAtomicI64Rmw32OrU:
				rmwOp = ssa.AtomicRmwOpOr
				switch atomicOp {
				case wasm.OpcodeAtomicI64RmwOr:
					size = 8
				case wasm.OpcodeAtomicI32RmwOr, wasm.OpcodeAtomicI64Rmw32OrU:
					size = 4
				case wasm.OpcodeAtomicI32Rmw16OrU, wasm.OpcodeAtomicI64Rmw16OrU:
					size = 2
				case wasm.OpcodeAtomicI32Rmw8OrU, wasm.OpcodeAtomicI64Rmw8OrU:
					size = 1
				}
			case wasm.OpcodeAtomicI32RmwXor, wasm.OpcodeAtomicI64RmwXor, wasm.OpcodeAtomicI32Rmw8XorU, wasm.OpcodeAtomicI32Rmw16XorU, wasm.OpcodeAtomicI64Rmw8XorU, wasm.OpcodeAtomicI64Rmw16XorU, wasm.OpcodeAtomicI64Rmw32XorU:
				rmwOp = ssa.AtomicRmwOpXor
				switch atomicOp {
				case wasm.OpcodeAtomicI64RmwXor:
					size = 8
				case wasm.OpcodeAtomicI32RmwXor, wasm.OpcodeAtomicI64Rmw32XorU:
					size = 4
				case wasm.OpcodeAtomicI32Rmw16XorU, wasm.OpcodeAtomicI64Rmw16XorU:
					size = 2
				case wasm.OpcodeAtomicI32Rmw8XorU, wasm.OpcodeAtomicI64Rmw8XorU:
					size = 1
				}
			case wasm.OpcodeAtomicI32RmwXchg, wasm.OpcodeAtomicI64RmwXchg, wasm.OpcodeAtomicI32Rmw8XchgU, wasm.OpcodeAtomicI32Rmw16XchgU, wasm.OpcodeAtomicI64Rmw8XchgU, wasm.OpcodeAtomicI64Rmw16XchgU, wasm.OpcodeAtomicI64Rmw32XchgU:
				rmwOp = ssa.AtomicRmwOpXchg
				switch atomicOp {
				case wasm.OpcodeAtomicI64RmwXchg:
					size = 8
				case wasm.OpcodeAtomicI32RmwXchg, wasm.OpcodeAtomicI64Rmw32XchgU:
					size = 4
				case wasm.OpcodeAtomicI32Rmw16XchgU, wasm.OpcodeAtomicI64Rmw16XchgU:
					size = 2
				case wasm.OpcodeAtomicI32Rmw8XchgU, wasm.OpcodeAtomicI64Rmw8XchgU:
					size = 1
				}
			}

			addr := c.atomicMemOpSetup(baseAddr, uint64(offset), size)
			res := builder.AllocateInstruction().AsAtomicRmw(rmwOp, addr, val, size).Insert(builder).Return()
			state.push(res)
		case wasm.OpcodeAtomicI32RmwCmpxchg, wasm.OpcodeAtomicI64RmwCmpxchg, wasm.OpcodeAtomicI32Rmw8CmpxchgU, wasm.OpcodeAtomicI32Rmw16CmpxchgU, wasm.OpcodeAtomicI64Rmw8CmpxchgU, wasm.OpcodeAtomicI64Rmw16CmpxchgU, wasm.OpcodeAtomicI64Rmw32CmpxchgU:
			_, offset := c.readMemArg()
			if state.unreachable {
				break
			}

			repl := state.pop()
			exp := state.pop()
			baseAddr := state.pop()

			var size uint64
			switch atomicOp {
			case wasm.OpcodeAtomicI64RmwCmpxchg:
				size = 8
			case wasm.OpcodeAtomicI32RmwCmpxchg, wasm.OpcodeAtomicI64Rmw32CmpxchgU:
				size = 4
			case wasm.OpcodeAtomicI32Rmw16CmpxchgU, wasm.OpcodeAtomicI64Rmw16CmpxchgU:
				size = 2
			case wasm.OpcodeAtomicI32Rmw8CmpxchgU, wasm.OpcodeAtomicI64Rmw8CmpxchgU:
				size = 1
			}
			addr := c.atomicMemOpSetup(baseAddr, uint64(offset), size)
			res := builder.AllocateInstruction().AsAtomicCas(addr, exp, repl, size).Insert(builder).Return()
			state.push(res)
		case wasm.OpcodeAtomicFence:
			order := c.readByte()
			if state.unreachable {
				break
			}
			if c.needMemory {
				builder.AllocateInstruction().AsFence(order).Insert(builder)
			}
		default:
			panic("TODO: unsupported atomic instruction: " + wasm.AtomicInstructionName(atomicOp))
		}
	case wasm.OpcodeRefFunc:
		funcIndex := c.readI32u()
		if state.unreachable {
			break
		}

		c.storeCallerModuleContext()

		funcIndexVal := builder.AllocateInstruction().AsIconst32(funcIndex).Insert(builder).Return()

		refFuncPtr := builder.AllocateInstruction().
			AsLoad(c.execCtxPtrValue,
				wazevoapi.ExecutionContextOffsetRefFuncTrampolineAddress.U32(),
				ssa.TypeI64,
			).Insert(builder).Return()

		args := c.allocateVarLengthValues(2, c.execCtxPtrValue, funcIndexVal)
		refFuncRet := builder.
			AllocateInstruction().
			AsCallIndirect(refFuncPtr, &c.refFuncSig, args).
			Insert(builder).Return()
		state.push(refFuncRet)

	case wasm.OpcodeRefNull:
		c.loweringState.pc++ // skips the reference type as we treat both of them as i64(0).
		if state.unreachable {
			break
		}
		ret := builder.AllocateInstruction().AsIconst64(0).Insert(builder).Return()
		state.push(ret)
	case wasm.OpcodeRefIsNull:
		if state.unreachable {
			break
		}
		r := state.pop()
		zero := builder.AllocateInstruction().AsIconst64(0).Insert(builder)
		icmp := builder.AllocateInstruction().
			AsIcmp(r, zero.Return(), ssa.IntegerCmpCondEqual).
			Insert(builder).
			Return()
		state.push(icmp)
	case wasm.OpcodeTableSet:
		tableIndex := c.readI32u()
		if state.unreachable {
			break
		}
		r := state.pop()
		targetOffsetInTable := state.pop()

		elementAddr := c.lowerAccessTableWithBoundsCheck(tableIndex, targetOffsetInTable)
		builder.AllocateInstruction().AsStore(ssa.OpcodeStore, r, elementAddr, 0).Insert(builder)

	case wasm.OpcodeTableGet:
		tableIndex := c.readI32u()
		if state.unreachable {
			break
		}
		targetOffsetInTable := state.pop()
		elementAddr := c.lowerAccessTableWithBoundsCheck(tableIndex, targetOffsetInTable)
		loaded := builder.AllocateInstruction().AsLoad(elementAddr, 0, ssa.TypeI64).Insert(builder).Return()
		state.push(loaded)
	default:
		panic("TODO: unsupported in wazevo yet: " + wasm.InstructionName(op))
	}

	if wazevoapi.FrontEndLoggingEnabled {
		fmt.Println("--------- Translated " + wasm.InstructionName(op) + " --------")
		fmt.Println("state: " + c.loweringState.String())
		fmt.Println(c.formatBuilder())
		fmt.Println("--------------------------")
	}
	c.loweringState.pc++
}

func (c *Compiler) lowerExtMul(v1, v2 ssa.Value, from, to ssa.VecLane, signed, low bool) ssa.Value {
	// TODO: The sequence `Widen; Widen; VIMul` can be substituted for a single instruction on some ISAs.
	builder := c.ssaBuilder

	v1lo := builder.AllocateInstruction().AsWiden(v1, from, signed, low).Insert(builder).Return()
	v2lo := builder.AllocateInstruction().AsWiden(v2, from, signed, low).Insert(builder).Return()

	return builder.AllocateInstruction().AsVImul(v1lo, v2lo, to).Insert(builder).Return()
}

const (
	tableInstanceBaseAddressOffset = 0
	tableInstanceLenOffset         = tableInstanceBaseAddressOffset + 8
)

func (c *Compiler) lowerAccessTableWithBoundsCheck(tableIndex uint32, elementOffsetInTable ssa.Value) (elementAddress ssa.Value) {
	builder := c.ssaBuilder

	// Load the table.
	loadTableInstancePtr := builder.AllocateInstruction()
	loadTableInstancePtr.AsLoad(c.moduleCtxPtrValue, c.offset.TableOffset(int(tableIndex)).U32(), ssa.TypeI64)
	builder.InsertInstruction(loadTableInstancePtr)
	tableInstancePtr := loadTableInstancePtr.Return()

	// Load the table's length.
	loadTableLen := builder.AllocateInstruction()
	loadTableLen.AsLoad(tableInstancePtr, tableInstanceLenOffset, ssa.TypeI32)
	builder.InsertInstruction(loadTableLen)
	tableLen := loadTableLen.Return()

	// Compare the length and the target, and trap if out of bounds.
	checkOOB := builder.AllocateInstruction()
	checkOOB.AsIcmp(elementOffsetInTable, tableLen, ssa.IntegerCmpCondUnsignedGreaterThanOrEqual)
	builder.InsertInstruction(checkOOB)
	exitIfOOB := builder.AllocateInstruction()
	exitIfOOB.AsExitIfTrueWithCode(c.execCtxPtrValue, checkOOB.Return(), wazevoapi.ExitCodeTableOutOfBounds)
	builder.InsertInstruction(exitIfOOB)

	// Get the base address of wasm.TableInstance.References.
	loadTableBaseAddress := builder.AllocateInstruction()
	loadTableBaseAddress.AsLoad(tableInstancePtr, tableInstanceBaseAddressOffset, ssa.TypeI64)
	builder.InsertInstruction(loadTableBaseAddress)
	tableBase := loadTableBaseAddress.Return()

	// Calculate the address of the target function. First we need to multiply targetOffsetInTable by 8 (pointer size).
	multiplyBy8 := builder.AllocateInstruction()
	three := builder.AllocateInstruction()
	three.AsIconst64(3)
	builder.InsertInstruction(three)
	multiplyBy8.AsIshl(elementOffsetInTable, three.Return())
	builder.InsertInstruction(multiplyBy8)
	targetOffsetInTableMultipliedBy8 := multiplyBy8.Return()

	// Then add the multiplied value to the base which results in the address of the target function (*wazevo.functionInstance)
	calcElementAddressInTable := builder.AllocateInstruction()
	calcElementAddressInTable.AsIadd(tableBase, targetOffsetInTableMultipliedBy8)
	builder.InsertInstruction(calcElementAddressInTable)
	return calcElementAddressInTable.Return()
}

func (c *Compiler) lowerCallIndirect(typeIndex, tableIndex uint32) {
	builder := c.ssaBuilder
	state := c.state()

	elementOffsetInTable := state.pop()
	functionInstancePtrAddress := c.lowerAccessTableWithBoundsCheck(tableIndex, elementOffsetInTable)
	loadFunctionInstancePtr := builder.AllocateInstruction()
	loadFunctionInstancePtr.AsLoad(functionInstancePtrAddress, 0, ssa.TypeI64)
	builder.InsertInstruction(loadFunctionInstancePtr)
	functionInstancePtr := loadFunctionInstancePtr.Return()

	// Check if it is not the null pointer.
	zero := builder.AllocateInstruction()
	zero.AsIconst64(0)
	builder.InsertInstruction(zero)
	checkNull := builder.AllocateInstruction()
	checkNull.AsIcmp(functionInstancePtr, zero.Return(), ssa.IntegerCmpCondEqual)
	builder.InsertInstruction(checkNull)
	exitIfNull := builder.AllocateInstruction()
	exitIfNull.AsExitIfTrueWithCode(c.execCtxPtrValue, checkNull.Return(), wazevoapi.ExitCodeIndirectCallNullPointer)
	builder.InsertInstruction(exitIfNull)

	// We need to do the type check. First, load the target function instance's typeID.
	loadTypeID := builder.AllocateInstruction()
	loadTypeID.AsLoad(functionInstancePtr, wazevoapi.FunctionInstanceTypeIDOffset, ssa.TypeI32)
	builder.InsertInstruction(loadTypeID)
	actualTypeID := loadTypeID.Return()

	// Next, we load the expected TypeID:
	loadTypeIDsBegin := builder.AllocateInstruction()
	loadTypeIDsBegin.AsLoad(c.moduleCtxPtrValue, c.offset.TypeIDs1stElement.U32(), ssa.TypeI64)
	builder.InsertInstruction(loadTypeIDsBegin)
	typeIDsBegin := loadTypeIDsBegin.Return()

	loadExpectedTypeID := builder.AllocateInstruction()
	loadExpectedTypeID.AsLoad(typeIDsBegin, uint32(typeIndex)*4 /* size of wasm.FunctionTypeID */, ssa.TypeI32)
	builder.InsertInstruction(loadExpectedTypeID)
	expectedTypeID := loadExpectedTypeID.Return()

	// Check if the type ID matches.
	checkTypeID := builder.AllocateInstruction()
	checkTypeID.AsIcmp(actualTypeID, expectedTypeID, ssa.IntegerCmpCondNotEqual)
	builder.InsertInstruction(checkTypeID)
	exitIfNotMatch := builder.AllocateInstruction()
	exitIfNotMatch.AsExitIfTrueWithCode(c.execCtxPtrValue, checkTypeID.Return(), wazevoapi.ExitCodeIndirectCallTypeMismatch)
	builder.InsertInstruction(exitIfNotMatch)

	// Now ready to call the function. Load the executable and moduleContextOpaquePtr from the function instance.
	loadExecutablePtr := builder.AllocateInstruction()
	loadExecutablePtr.AsLoad(functionInstancePtr, wazevoapi.FunctionInstanceExecutableOffset, ssa.TypeI64)
	builder.InsertInstruction(loadExecutablePtr)
	executablePtr := loadExecutablePtr.Return()
	loadModuleContextOpaquePtr := builder.AllocateInstruction()
	loadModuleContextOpaquePtr.AsLoad(functionInstancePtr, wazevoapi.FunctionInstanceModuleContextOpaquePtrOffset, ssa.TypeI64)
	builder.InsertInstruction(loadModuleContextOpaquePtr)
	moduleContextOpaquePtr := loadModuleContextOpaquePtr.Return()

	typ := &c.m.TypeSection[typeIndex]
	tail := len(state.values) - len(typ.Params)
	vs := state.values[tail:]
	state.values = state.values[:tail]
	args := c.allocateVarLengthValues(2+len(vs), c.execCtxPtrValue, moduleContextOpaquePtr)
	args = args.Append(builder.VarLengthPool(), vs...)

	// Before transfer the control to the callee, we have to store the current module's moduleContextPtr
	// into execContext.callerModuleContextPtr in case when the callee is a Go function.
	c.storeCallerModuleContext()

	call := builder.AllocateInstruction()
	call.AsCallIndirect(executablePtr, c.signatures[typ], args)
	builder.InsertInstruction(call)

	first, rest := call.Returns()
	if first.Valid() {
		state.push(first)
	}
	for _, v := range rest {
		state.push(v)
	}

	c.reloadAfterCall()
}

// memOpSetup inserts the bounds check and calculates the address of the memory operation (loads/stores).
func (c *Compiler) memOpSetup(baseAddr ssa.Value, constOffset, operationSizeInBytes uint64) (address ssa.Value) {
	address = ssa.ValueInvalid
	builder := c.ssaBuilder

	baseAddrID := baseAddr.ID()
	ceil := constOffset + operationSizeInBytes
	if known := c.getKnownSafeBound(baseAddrID); known.valid() {
		// We reuse the calculated absolute address even if the bound is not known to be safe.
		address = known.absoluteAddr
		if ceil <= known.bound {
			if !address.Valid() {
				// This means that, the bound is known to be safe, but the memory base might have changed.
				// So, we re-calculate the address.
				memBase := c.getMemoryBaseValue(false)
				extBaseAddr := builder.AllocateInstruction().
					AsUExtend(baseAddr, 32, 64).
					Insert(builder).
					Return()
				address = builder.AllocateInstruction().
					AsIadd(memBase, extBaseAddr).Insert(builder).Return()
				known.absoluteAddr = address // Update the absolute address for the subsequent memory access.
			}
			return
		}
	}

	ceilConst := builder.AllocateInstruction()
	ceilConst.AsIconst64(ceil)
	builder.InsertInstruction(ceilConst)

	// We calculate the offset in 64-bit space.
	extBaseAddr := builder.AllocateInstruction().
		AsUExtend(baseAddr, 32, 64).
		Insert(builder).
		Return()

	// Note: memLen is already zero extended to 64-bit space at the load time.
	memLen := c.getMemoryLenValue(false)

	// baseAddrPlusCeil = baseAddr + ceil
	baseAddrPlusCeil := builder.AllocateInstruction()
	baseAddrPlusCeil.AsIadd(extBaseAddr, ceilConst.Return())
	builder.InsertInstruction(baseAddrPlusCeil)

	// Check for out of bounds memory access: `memLen >= baseAddrPlusCeil`.
	cmp := builder.AllocateInstruction()
	cmp.AsIcmp(memLen, baseAddrPlusCeil.Return(), ssa.IntegerCmpCondUnsignedLessThan)
	builder.InsertInstruction(cmp)
	exitIfNZ := builder.AllocateInstruction()
	exitIfNZ.AsExitIfTrueWithCode(c.execCtxPtrValue, cmp.Return(), wazevoapi.ExitCodeMemoryOutOfBounds)
	builder.InsertInstruction(exitIfNZ)

	// Load the value from memBase + extBaseAddr.
	if address == ssa.ValueInvalid { // Reuse the value if the memBase is already calculated at this point.
		memBase := c.getMemoryBaseValue(false)
		address = builder.AllocateInstruction().
			AsIadd(memBase, extBaseAddr).Insert(builder).Return()
	}

	// Record the bound ceil for this baseAddr is known to be safe for the subsequent memory access in the same block.
	c.recordKnownSafeBound(baseAddrID, ceil, address)
	return
}

// atomicMemOpSetup inserts the bounds check and calculates the address of the memory operation (loads/stores), including
// the constant offset and performs an alignment check on the final address.
func (c *Compiler) atomicMemOpSetup(baseAddr ssa.Value, constOffset, operationSizeInBytes uint64) (address ssa.Value) {
	builder := c.ssaBuilder

	addrWithoutOffset := c.memOpSetup(baseAddr, constOffset, operationSizeInBytes)
	var addr ssa.Value
	if constOffset == 0 {
		addr = addrWithoutOffset
	} else {
		offset := builder.AllocateInstruction().AsIconst64(constOffset).Insert(builder).Return()
		addr = builder.AllocateInstruction().AsIadd(addrWithoutOffset, offset).Insert(builder).Return()
	}

	c.memAlignmentCheck(addr, operationSizeInBytes)

	return addr
}

func (c *Compiler) memAlignmentCheck(addr ssa.Value, operationSizeInBytes uint64) {
	if operationSizeInBytes == 1 {
		return // No alignment restrictions when accessing a byte
	}
	var checkBits uint64
	switch operationSizeInBytes {
	case 2:
		checkBits = 0b1
	case 4:
		checkBits = 0b11
	case 8:
		checkBits = 0b111
	}

	builder := c.ssaBuilder

	mask := builder.AllocateInstruction().AsIconst64(checkBits).Insert(builder).Return()
	masked := builder.AllocateInstruction().AsBand(addr, mask).Insert(builder).Return()
	zero := builder.AllocateInstruction().AsIconst64(0).Insert(builder).Return()
	cmp := builder.AllocateInstruction().AsIcmp(masked, zero, ssa.IntegerCmpCondNotEqual).Insert(builder).Return()
	builder.AllocateInstruction().AsExitIfTrueWithCode(c.execCtxPtrValue, cmp, wazevoapi.ExitCodeUnalignedAtomic).Insert(builder)
}

func (c *Compiler) callMemmove(dst, src, size ssa.Value) {
	args := c.allocateVarLengthValues(3, dst, src, size)
	if size.Type() != ssa.TypeI64 {
		panic("TODO: memmove size must be i64")
	}

	builder := c.ssaBuilder
	memmovePtr := builder.AllocateInstruction().
		AsLoad(c.execCtxPtrValue,
			wazevoapi.ExecutionContextOffsetMemmoveAddress.U32(),
			ssa.TypeI64,
		).Insert(builder).Return()
	builder.AllocateInstruction().AsCallGoRuntimeMemmove(memmovePtr, &c.memmoveSig, args).Insert(builder)
}

func (c *Compiler) reloadAfterCall() {
	// Note that when these are not used in the following instructions, they will be optimized out.
	// So in any ways, we define them!

	// After calling any function, memory buffer might have changed. So we need to re-define the variable.
	// However, if the memory is shared, we don't need to reload the memory base and length as the base will never change.
	if c.needMemory && !c.memoryShared {
		c.reloadMemoryBaseLen()
	}

	// Also, any mutable Global can change.
	for _, index := range c.mutableGlobalVariablesIndexes {
		_ = c.getWasmGlobalValue(index, true)
	}
}

func (c *Compiler) reloadMemoryBaseLen() {
	_ = c.getMemoryBaseValue(true)
	_ = c.getMemoryLenValue(true)

	// This function being called means that the memory base might have changed.
	// Therefore, we need to clear the absolute addresses recorded in the known safe bounds
	// because we cache the absolute address of the memory access per each base offset.
	c.resetAbsoluteAddressInSafeBounds()
}

func (c *Compiler) setWasmGlobalValue(index wasm.Index, v ssa.Value) {
	variable := c.globalVariables[index]
	opaqueOffset := c.offset.GlobalInstanceOffset(index)

	builder := c.ssaBuilder
	if index < c.m.ImportGlobalCount {
		loadGlobalInstPtr := builder.AllocateInstruction()
		loadGlobalInstPtr.AsLoad(c.moduleCtxPtrValue, uint32(opaqueOffset), ssa.TypeI64)
		builder.InsertInstruction(loadGlobalInstPtr)

		store := builder.AllocateInstruction()
		store.AsStore(ssa.OpcodeStore, v, loadGlobalInstPtr.Return(), uint32(0))
		builder.InsertInstruction(store)

	} else {
		store := builder.AllocateInstruction()
		store.AsStore(ssa.OpcodeStore, v, c.moduleCtxPtrValue, uint32(opaqueOffset))
		builder.InsertInstruction(store)
	}

	// The value has changed to `v`, so we record it.
	builder.DefineVariableInCurrentBB(variable, v)
}

func (c *Compiler) getWasmGlobalValue(index wasm.Index, forceLoad bool) ssa.Value {
	variable := c.globalVariables[index]
	typ := c.globalVariablesTypes[index]
	opaqueOffset := c.offset.GlobalInstanceOffset(index)

	builder := c.ssaBuilder
	if !forceLoad {
		if v := builder.FindValueInLinearPath(variable); v.Valid() {
			return v
		}
	}

	var load *ssa.Instruction
	if index < c.m.ImportGlobalCount {
		loadGlobalInstPtr := builder.AllocateInstruction()
		loadGlobalInstPtr.AsLoad(c.moduleCtxPtrValue, uint32(opaqueOffset), ssa.TypeI64)
		builder.InsertInstruction(loadGlobalInstPtr)
		load = builder.AllocateInstruction().
			AsLoad(loadGlobalInstPtr.Return(), uint32(0), typ)
	} else {
		load = builder.AllocateInstruction().
			AsLoad(c.moduleCtxPtrValue, uint32(opaqueOffset), typ)
	}

	v := load.Insert(builder).Return()
	builder.DefineVariableInCurrentBB(variable, v)
	return v
}

const (
	memoryInstanceBufOffset     = 0
	memoryInstanceBufSizeOffset = memoryInstanceBufOffset + 8
)

func (c *Compiler) getMemoryBaseValue(forceReload bool) ssa.Value {
	builder := c.ssaBuilder
	variable := c.memoryBaseVariable
	if !forceReload {
		if v := builder.FindValueInLinearPath(variable); v.Valid() {
			return v
		}
	}

	var ret ssa.Value
	if c.offset.LocalMemoryBegin < 0 {
		loadMemInstPtr := builder.AllocateInstruction()
		loadMemInstPtr.AsLoad(c.moduleCtxPtrValue, c.offset.ImportedMemoryBegin.U32(), ssa.TypeI64)
		builder.InsertInstruction(loadMemInstPtr)
		memInstPtr := loadMemInstPtr.Return()

		loadBufPtr := builder.AllocateInstruction()
		loadBufPtr.AsLoad(memInstPtr, memoryInstanceBufOffset, ssa.TypeI64)
		builder.InsertInstruction(loadBufPtr)
		ret = loadBufPtr.Return()
	} else {
		load := builder.AllocateInstruction()
		load.AsLoad(c.moduleCtxPtrValue, c.offset.LocalMemoryBase().U32(), ssa.TypeI64)
		builder.InsertInstruction(load)
		ret = load.Return()
	}

	builder.DefineVariableInCurrentBB(variable, ret)
	return ret
}

func (c *Compiler) getMemoryLenValue(forceReload bool) ssa.Value {
	variable := c.memoryLenVariable
	builder := c.ssaBuilder
	if !forceReload && !c.memoryShared {
		if v := builder.FindValueInLinearPath(variable); v.Valid() {
			return v
		}
	}

	var ret ssa.Value
	if c.offset.LocalMemoryBegin < 0 {
		loadMemInstPtr := builder.AllocateInstruction()
		loadMemInstPtr.AsLoad(c.moduleCtxPtrValue, c.offset.ImportedMemoryBegin.U32(), ssa.TypeI64)
		builder.InsertInstruction(loadMemInstPtr)
		memInstPtr := loadMemInstPtr.Return()

		loadBufSizePtr := builder.AllocateInstruction()
		if c.memoryShared {
			sizeOffset := builder.AllocateInstruction().AsIconst64(memoryInstanceBufSizeOffset).Insert(builder).Return()
			addr := builder.AllocateInstruction().AsIadd(memInstPtr, sizeOffset).Insert(builder).Return()
			loadBufSizePtr.AsAtomicLoad(addr, 8, ssa.TypeI64)
		} else {
			loadBufSizePtr.AsLoad(memInstPtr, memoryInstanceBufSizeOffset, ssa.TypeI64)
		}
		builder.InsertInstruction(loadBufSizePtr)

		ret = loadBufSizePtr.Return()
	} else {
		load := builder.AllocateInstruction()
		if c.memoryShared {
			lenOffset := builder.AllocateInstruction().AsIconst64(c.offset.LocalMemoryLen().U64()).Insert(builder).Return()
			addr := builder.AllocateInstruction().AsIadd(c.moduleCtxPtrValue, lenOffset).Insert(builder).Return()
			load.AsAtomicLoad(addr, 8, ssa.TypeI64)
		} else {
			load.AsExtLoad(ssa.OpcodeUload32, c.moduleCtxPtrValue, c.offset.LocalMemoryLen().U32(), true)
		}
		builder.InsertInstruction(load)
		ret = load.Return()
	}

	builder.DefineVariableInCurrentBB(variable, ret)
	return ret
}

func (c *Compiler) insertIcmp(cond ssa.IntegerCmpCond) {
	state, builder := c.state(), c.ssaBuilder
	y, x := state.pop(), state.pop()
	cmp := builder.AllocateInstruction()
	cmp.AsIcmp(x, y, cond)
	builder.InsertInstruction(cmp)
	value := cmp.Return()
	state.push(value)
}

func (c *Compiler) insertFcmp(cond ssa.FloatCmpCond) {
	state, builder := c.state(), c.ssaBuilder
	y, x := state.pop(), state.pop()
	cmp := builder.AllocateInstruction()
	cmp.AsFcmp(x, y, cond)
	builder.InsertInstruction(cmp)
	value := cmp.Return()
	state.push(value)
}

// storeCallerModuleContext stores the current module's moduleContextPtr into execContext.callerModuleContextPtr.
func (c *Compiler) storeCallerModuleContext() {
	builder := c.ssaBuilder
	execCtx := c.execCtxPtrValue
	store := builder.AllocateInstruction()
	store.AsStore(ssa.OpcodeStore,
		c.moduleCtxPtrValue, execCtx, wazevoapi.ExecutionContextOffsetCallerModuleContextPtr.U32())
	builder.InsertInstruction(store)
}

func (c *Compiler) readByte() byte {
	v := c.wasmFunctionBody[c.loweringState.pc+1]
	c.loweringState.pc++
	return v
}

func (c *Compiler) readI32u() uint32 {
	v, n, err := leb128.LoadUint32(c.wasmFunctionBody[c.loweringState.pc+1:])
	if err != nil {
		panic(err) // shouldn't be reached since compilation comes after validation.
	}
	c.loweringState.pc += int(n)
	return v
}

func (c *Compiler) readI32s() int32 {
	v, n, err := leb128.LoadInt32(c.wasmFunctionBody[c.loweringState.pc+1:])
	if err != nil {
		panic(err) // shouldn't be reached since compilation comes after validation.
	}
	c.loweringState.pc += int(n)
	return v
}

func (c *Compiler) readI64s() int64 {
	v, n, err := leb128.LoadInt64(c.wasmFunctionBody[c.loweringState.pc+1:])
	if err != nil {
		panic(err) // shouldn't be reached since compilation comes after validation.
	}
	c.loweringState.pc += int(n)
	return v
}

func (c *Compiler) readF32() float32 {
	v := math.Float32frombits(binary.LittleEndian.Uint32(c.wasmFunctionBody[c.loweringState.pc+1:]))
	c.loweringState.pc += 4
	return v
}

func (c *Compiler) readF64() float64 {
	v := math.Float64frombits(binary.LittleEndian.Uint64(c.wasmFunctionBody[c.loweringState.pc+1:]))
	c.loweringState.pc += 8
	return v
}

// readBlockType reads the block type from the current position of the bytecode reader.
func (c *Compiler) readBlockType() *wasm.FunctionType {
	state := c.state()

	c.br.Reset(c.wasmFunctionBody[state.pc+1:])
	bt, num, err := wasm.DecodeBlockType(c.m.TypeSection, c.br, api.CoreFeaturesV2)
	if err != nil {
		panic(err) // shouldn't be reached since compilation comes after validation.
	}
	state.pc += int(num)

	return bt
}

func (c *Compiler) readMemArg() (align, offset uint32) {
	state := c.state()

	align, num, err := leb128.LoadUint32(c.wasmFunctionBody[state.pc+1:])
	if err != nil {
		panic(fmt.Errorf("read memory align: %v", err))
	}

	state.pc += int(num)
	offset, num, err = leb128.LoadUint32(c.wasmFunctionBody[state.pc+1:])
	if err != nil {
		panic(fmt.Errorf("read memory offset: %v", err))
	}

	state.pc += int(num)
	return align, offset
}

// insertJumpToBlock inserts a jump instruction to the given block in the current block.
func (c *Compiler) insertJumpToBlock(args ssa.Values, targetBlk ssa.BasicBlock) {
	if targetBlk.ReturnBlock() {
		if c.needListener {
			c.callListenerAfter()
		}
	}

	builder := c.ssaBuilder
	jmp := builder.AllocateInstruction()
	jmp.AsJump(args, targetBlk)
	builder.InsertInstruction(jmp)
}

func (c *Compiler) insertIntegerExtend(signed bool, from, to byte) {
	state := c.state()
	builder := c.ssaBuilder
	v := state.pop()
	extend := builder.AllocateInstruction()
	if signed {
		extend.AsSExtend(v, from, to)
	} else {
		extend.AsUExtend(v, from, to)
	}
	builder.InsertInstruction(extend)
	value := extend.Return()
	state.push(value)
}

func (c *Compiler) switchTo(originalStackLen int, targetBlk ssa.BasicBlock) {
	if targetBlk.Preds() == 0 {
		c.loweringState.unreachable = true
	}

	// Now we should adjust the stack and start translating the continuation block.
	c.loweringState.values = c.loweringState.values[:originalStackLen]

	c.ssaBuilder.SetCurrentBlock(targetBlk)

	// At this point, blocks params consist only of the Wasm-level parameters,
	// (since it's added only when we are trying to resolve variable *inside* this block).
	for i := 0; i < targetBlk.Params(); i++ {
		value := targetBlk.Param(i)
		c.loweringState.push(value)
	}
}

// results returns the number of results of the current function.
func (c *Compiler) results() int {
	return len(c.wasmFunctionTyp.Results)
}

func (c *Compiler) lowerBrTable(labels []uint32, index ssa.Value) {
	state := c.state()
	builder := c.ssaBuilder

	f := state.ctrlPeekAt(int(labels[0]))
	var numArgs int
	if f.isLoop() {
		numArgs = len(f.blockType.Params)
	} else {
		numArgs = len(f.blockType.Results)
	}

	varPool := builder.VarLengthPool()
	trampolineBlockIDs := varPool.Allocate(len(labels))

	// We need trampoline blocks since depending on the target block structure, we might end up inserting moves before jumps,
	// which cannot be done with br_table. Instead, we can do such per-block moves in the trampoline blocks.
	// At the linking phase (very end of the backend), we can remove the unnecessary jumps, and therefore no runtime overhead.
	currentBlk := builder.CurrentBlock()
	for _, l := range labels {
		// Args are always on the top of the stack. Note that we should not share the args slice
		// among the jump instructions since the args are modified during passes (e.g. redundant phi elimination).
		args := c.nPeekDup(numArgs)
		targetBlk, _ := state.brTargetArgNumFor(l)
		trampoline := builder.AllocateBasicBlock()
		builder.SetCurrentBlock(trampoline)
		c.insertJumpToBlock(args, targetBlk)
		trampolineBlockIDs = trampolineBlockIDs.Append(builder.VarLengthPool(), ssa.Value(trampoline.ID()))
	}
	builder.SetCurrentBlock(currentBlk)

	// If the target block has no arguments, we can just jump to the target block.
	brTable := builder.AllocateInstruction()
	brTable.AsBrTable(index, trampolineBlockIDs)
	builder.InsertInstruction(brTable)

	for _, trampolineID := range trampolineBlockIDs.View() {
		builder.Seal(builder.BasicBlock(ssa.BasicBlockID(trampolineID)))
	}
}

func (l *loweringState) brTargetArgNumFor(labelIndex uint32) (targetBlk ssa.BasicBlock, argNum int) {
	targetFrame := l.ctrlPeekAt(int(labelIndex))
	if targetFrame.isLoop() {
		targetBlk, argNum = targetFrame.blk, len(targetFrame.blockType.Params)
	} else {
		targetBlk, argNum = targetFrame.followingBlock, len(targetFrame.blockType.Results)
	}
	return
}

func (c *Compiler) callListenerBefore() {
	c.storeCallerModuleContext()

	builder := c.ssaBuilder
	beforeListeners1stElement := builder.AllocateInstruction().
		AsLoad(c.moduleCtxPtrValue,
			c.offset.BeforeListenerTrampolines1stElement.U32(),
			ssa.TypeI64,
		).Insert(builder).Return()

	beforeListenerPtr := builder.AllocateInstruction().
		AsLoad(beforeListeners1stElement, uint32(c.wasmFunctionTypeIndex)*8 /* 8 bytes per index */, ssa.TypeI64).Insert(builder).Return()

	entry := builder.EntryBlock()
	ps := entry.Params()

	args := c.allocateVarLengthValues(ps, c.execCtxPtrValue,
		builder.AllocateInstruction().AsIconst32(c.wasmLocalFunctionIndex).Insert(builder).Return())
	for i := 2; i < ps; i++ {
		args = args.Append(builder.VarLengthPool(), entry.Param(i))
	}

	beforeSig := c.listenerSignatures[c.wasmFunctionTyp][0]
	builder.AllocateInstruction().
		AsCallIndirect(beforeListenerPtr, beforeSig, args).
		Insert(builder)
}

func (c *Compiler) callListenerAfter() {
	c.storeCallerModuleContext()

	builder := c.ssaBuilder
	afterListeners1stElement := builder.AllocateInstruction().
		AsLoad(c.moduleCtxPtrValue,
			c.offset.AfterListenerTrampolines1stElement.U32(),
			ssa.TypeI64,
		).Insert(builder).Return()

	afterListenerPtr := builder.AllocateInstruction().
		AsLoad(afterListeners1stElement,
			uint32(c.wasmFunctionTypeIndex)*8 /* 8 bytes per index */, ssa.TypeI64).
		Insert(builder).
		Return()

	afterSig := c.listenerSignatures[c.wasmFunctionTyp][1]
	args := c.allocateVarLengthValues(
		c.results()+2,
		c.execCtxPtrValue,
		builder.AllocateInstruction().AsIconst32(c.wasmLocalFunctionIndex).Insert(builder).Return(),
	)

	l := c.state()
	tail := len(l.values)
	args = args.Append(c.ssaBuilder.VarLengthPool(), l.values[tail-c.results():tail]...)
	builder.AllocateInstruction().
		AsCallIndirect(afterListenerPtr, afterSig, args).
		Insert(builder)
}

const (
	elementOrDataInstanceLenOffset = 8
	elementOrDataInstanceSize      = 24
)

// dropInstance inserts instructions to drop the element/data instance specified by the given index.
func (c *Compiler) dropDataOrElementInstance(index uint32, firstItemOffset wazevoapi.Offset) {
	builder := c.ssaBuilder
	instPtr := c.dataOrElementInstanceAddr(index, firstItemOffset)

	zero := builder.AllocateInstruction().AsIconst64(0).Insert(builder).Return()

	// Clear the instance.
	builder.AllocateInstruction().AsStore(ssa.OpcodeStore, zero, instPtr, 0).Insert(builder)
	builder.AllocateInstruction().AsStore(ssa.OpcodeStore, zero, instPtr, elementOrDataInstanceLenOffset).Insert(builder)
	builder.AllocateInstruction().AsStore(ssa.OpcodeStore, zero, instPtr, elementOrDataInstanceLenOffset+8).Insert(builder)
}

func (c *Compiler) dataOrElementInstanceAddr(index uint32, firstItemOffset wazevoapi.Offset) ssa.Value {
	builder := c.ssaBuilder

	_1stItemPtr := builder.
		AllocateInstruction().
		AsLoad(c.moduleCtxPtrValue, firstItemOffset.U32(), ssa.TypeI64).
		Insert(builder).Return()

	// Each data/element instance is a slice, so we need to multiply index by 16 to get the offset of the target instance.
	index = index * elementOrDataInstanceSize
	indexExt := builder.AllocateInstruction().AsIconst64(uint64(index)).Insert(builder).Return()
	// Then, add the offset to the address of the instance.
	instPtr := builder.AllocateInstruction().AsIadd(_1stItemPtr, indexExt).Insert(builder).Return()
	return instPtr
}

func (c *Compiler) boundsCheckInDataOrElementInstance(instPtr, offsetInInstance, copySize ssa.Value, exitCode wazevoapi.ExitCode) {
	builder := c.ssaBuilder
	dataInstLen := builder.AllocateInstruction().
		AsLoad(instPtr, elementOrDataInstanceLenOffset, ssa.TypeI64).
		Insert(builder).Return()
	ceil := builder.AllocateInstruction().AsIadd(offsetInInstance, copySize).Insert(builder).Return()
	cmp := builder.AllocateInstruction().
		AsIcmp(dataInstLen, ceil, ssa.IntegerCmpCondUnsignedLessThan).
		Insert(builder).
		Return()
	builder.AllocateInstruction().
		AsExitIfTrueWithCode(c.execCtxPtrValue, cmp, exitCode).
		Insert(builder)
}

func (c *Compiler) boundsCheckInTable(tableIndex uint32, offset, size ssa.Value) (tableInstancePtr ssa.Value) {
	builder := c.ssaBuilder
	dstCeil := builder.AllocateInstruction().AsIadd(offset, size).Insert(builder).Return()

	// Load the table.
	tableInstancePtr = builder.AllocateInstruction().
		AsLoad(c.moduleCtxPtrValue, c.offset.TableOffset(int(tableIndex)).U32(), ssa.TypeI64).
		Insert(builder).Return()

	// Load the table's length.
	tableLen := builder.AllocateInstruction().
		AsLoad(tableInstancePtr, tableInstanceLenOffset, ssa.TypeI32).Insert(builder).Return()
	tableLenExt := builder.AllocateInstruction().AsUExtend(tableLen, 32, 64).Insert(builder).Return()

	// Compare the length and the target, and trap if out of bounds.
	checkOOB := builder.AllocateInstruction()
	checkOOB.AsIcmp(tableLenExt, dstCeil, ssa.IntegerCmpCondUnsignedLessThan)
	builder.InsertInstruction(checkOOB)
	exitIfOOB := builder.AllocateInstruction()
	exitIfOOB.AsExitIfTrueWithCode(c.execCtxPtrValue, checkOOB.Return(), wazevoapi.ExitCodeTableOutOfBounds)
	builder.InsertInstruction(exitIfOOB)
	return
}

func (c *Compiler) loadTableBaseAddr(tableInstancePtr ssa.Value) ssa.Value {
	builder := c.ssaBuilder
	loadTableBaseAddress := builder.
		AllocateInstruction().
		AsLoad(tableInstancePtr, tableInstanceBaseAddressOffset, ssa.TypeI64).
		Insert(builder)
	return loadTableBaseAddress.Return()
}

func (c *Compiler) boundsCheckInMemory(memLen, offset, size ssa.Value) {
	builder := c.ssaBuilder
	ceil := builder.AllocateInstruction().AsIadd(offset, size).Insert(builder).Return()
	cmp := builder.AllocateInstruction().
		AsIcmp(memLen, ceil, ssa.IntegerCmpCondUnsignedLessThan).
		Insert(builder).
		Return()
	builder.AllocateInstruction().
		AsExitIfTrueWithCode(c.execCtxPtrValue, cmp, wazevoapi.ExitCodeMemoryOutOfBounds).
		Insert(builder)
}
