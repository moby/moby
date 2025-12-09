package backend

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

// NewCompiler returns a new Compiler that can generate a machine code.
func NewCompiler(ctx context.Context, mach Machine, builder ssa.Builder) Compiler {
	return newCompiler(ctx, mach, builder)
}

func newCompiler(_ context.Context, mach Machine, builder ssa.Builder) *compiler {
	argResultInts, argResultFloats := mach.ArgsResultsRegs()
	c := &compiler{
		mach: mach, ssaBuilder: builder,
		nextVRegID:      regalloc.VRegIDNonReservedBegin,
		argResultInts:   argResultInts,
		argResultFloats: argResultFloats,
	}
	mach.SetCompiler(c)
	return c
}

// Compiler is the backend of wazevo which takes ssa.Builder and Machine,
// use the information there to emit the final machine code.
type Compiler interface {
	// SSABuilder returns the ssa.Builder used by this compiler.
	SSABuilder() ssa.Builder

	// Compile executes the following steps:
	// 	1. Lower()
	// 	2. RegAlloc()
	// 	3. Finalize()
	// 	4. Encode()
	//
	// Each step can be called individually for testing purpose, therefore they are exposed in this interface too.
	//
	// The returned byte slices are the machine code and the relocation information for the machine code.
	// The caller is responsible for copying them immediately since the compiler may reuse the buffer.
	Compile(ctx context.Context) (_ []byte, _ []RelocationInfo, _ error)

	// Lower lowers the given ssa.Instruction to the machine-specific instructions.
	Lower()

	// RegAlloc performs the register allocation after Lower is called.
	RegAlloc()

	// Finalize performs the finalization of the compilation, including machine code emission.
	// This must be called after RegAlloc.
	Finalize(ctx context.Context) error

	// Buf returns the buffer of the encoded machine code. This is only used for testing purpose.
	Buf() []byte

	BufPtr() *[]byte

	// Format returns the debug string of the current state of the compiler.
	Format() string

	// Init initializes the internal state of the compiler for the next compilation.
	Init()

	// AllocateVReg allocates a new virtual register of the given type.
	AllocateVReg(typ ssa.Type) regalloc.VReg

	// ValueDefinition returns the definition of the given value.
	ValueDefinition(ssa.Value) SSAValueDefinition

	// VRegOf returns the virtual register of the given ssa.Value.
	VRegOf(value ssa.Value) regalloc.VReg

	// TypeOf returns the ssa.Type of the given virtual register.
	TypeOf(regalloc.VReg) ssa.Type

	// MatchInstr returns true if the given definition is from an instruction with the given opcode, the current group ID,
	// and a refcount of 1. That means, the instruction can be merged/swapped within the current instruction group.
	MatchInstr(def SSAValueDefinition, opcode ssa.Opcode) bool

	// MatchInstrOneOf is the same as MatchInstr but for multiple opcodes. If it matches one of ssa.Opcode,
	// this returns the opcode. Otherwise, this returns ssa.OpcodeInvalid.
	//
	// Note: caller should be careful to avoid excessive allocation on opcodes slice.
	MatchInstrOneOf(def SSAValueDefinition, opcodes []ssa.Opcode) ssa.Opcode

	// AddRelocationInfo appends the relocation information for the function reference at the current buffer offset.
	AddRelocationInfo(funcRef ssa.FuncRef)

	// AddSourceOffsetInfo appends the source offset information for the given offset.
	AddSourceOffsetInfo(executableOffset int64, sourceOffset ssa.SourceOffset)

	// SourceOffsetInfo returns the source offset information for the current buffer offset.
	SourceOffsetInfo() []SourceOffsetInfo

	// EmitByte appends a byte to the buffer. Used during the code emission.
	EmitByte(b byte)

	// Emit4Bytes appends 4 bytes to the buffer. Used during the code emission.
	Emit4Bytes(b uint32)

	// Emit8Bytes appends 8 bytes to the buffer. Used during the code emission.
	Emit8Bytes(b uint64)

	// GetFunctionABI returns the ABI information for the given signature.
	GetFunctionABI(sig *ssa.Signature) *FunctionABI
}

// RelocationInfo represents the relocation information for a call instruction.
type RelocationInfo struct {
	// Offset represents the offset from the beginning of the machine code of either a function or the entire module.
	Offset int64
	// Target is the target function of the call instruction.
	FuncRef ssa.FuncRef
}

// compiler implements Compiler.
type compiler struct {
	mach       Machine
	currentGID ssa.InstructionGroupID
	ssaBuilder ssa.Builder
	// nextVRegID is the next virtual register ID to be allocated.
	nextVRegID regalloc.VRegID
	// ssaValueToVRegs maps ssa.ValueID to regalloc.VReg.
	ssaValueToVRegs [] /* VRegID to */ regalloc.VReg
	ssaValuesInfo   []ssa.ValueInfo
	// returnVRegs is the list of virtual registers that store the return values.
	returnVRegs  []regalloc.VReg
	varEdges     [][2]regalloc.VReg
	varEdgeTypes []ssa.Type
	constEdges   []struct {
		cInst *ssa.Instruction
		dst   regalloc.VReg
	}
	vRegSet         []bool
	vRegIDs         []regalloc.VRegID
	tempRegs        []regalloc.VReg
	tmpVals         []ssa.Value
	ssaTypeOfVRegID [] /* VRegID to */ ssa.Type
	buf             []byte
	relocations     []RelocationInfo
	sourceOffsets   []SourceOffsetInfo
	// abis maps ssa.SignatureID to the ABI implementation.
	abis                           []FunctionABI
	argResultInts, argResultFloats []regalloc.RealReg
}

// SourceOffsetInfo is a data to associate the source offset with the executable offset.
type SourceOffsetInfo struct {
	// SourceOffset is the source offset in the original source code.
	SourceOffset ssa.SourceOffset
	// ExecutableOffset is the offset in the compiled executable.
	ExecutableOffset int64
}

// Compile implements Compiler.Compile.
func (c *compiler) Compile(ctx context.Context) ([]byte, []RelocationInfo, error) {
	c.Lower()
	if wazevoapi.PrintSSAToBackendIRLowering && wazevoapi.PrintEnabledIndex(ctx) {
		fmt.Printf("[[[after lowering for %s ]]]%s\n", wazevoapi.GetCurrentFunctionName(ctx), c.Format())
	}
	if wazevoapi.DeterministicCompilationVerifierEnabled {
		wazevoapi.VerifyOrSetDeterministicCompilationContextValue(ctx, "After lowering to ISA specific IR", c.Format())
	}
	c.RegAlloc()
	if wazevoapi.PrintRegisterAllocated && wazevoapi.PrintEnabledIndex(ctx) {
		fmt.Printf("[[[after regalloc for %s]]]%s\n", wazevoapi.GetCurrentFunctionName(ctx), c.Format())
	}
	if wazevoapi.DeterministicCompilationVerifierEnabled {
		wazevoapi.VerifyOrSetDeterministicCompilationContextValue(ctx, "After Register Allocation", c.Format())
	}
	if err := c.Finalize(ctx); err != nil {
		return nil, nil, err
	}
	if wazevoapi.PrintFinalizedMachineCode && wazevoapi.PrintEnabledIndex(ctx) {
		fmt.Printf("[[[after finalize for %s]]]%s\n", wazevoapi.GetCurrentFunctionName(ctx), c.Format())
	}
	if wazevoapi.DeterministicCompilationVerifierEnabled {
		wazevoapi.VerifyOrSetDeterministicCompilationContextValue(ctx, "After Finalization", c.Format())
	}
	return c.buf, c.relocations, nil
}

// RegAlloc implements Compiler.RegAlloc.
func (c *compiler) RegAlloc() {
	c.mach.RegAlloc()
}

// Finalize implements Compiler.Finalize.
func (c *compiler) Finalize(ctx context.Context) error {
	c.mach.PostRegAlloc()
	return c.mach.Encode(ctx)
}

// setCurrentGroupID sets the current instruction group ID.
func (c *compiler) setCurrentGroupID(gid ssa.InstructionGroupID) {
	c.currentGID = gid
}

// assignVirtualRegisters assigns a virtual register to each ssa.ValueID Valid in the ssa.Builder.
func (c *compiler) assignVirtualRegisters() {
	builder := c.ssaBuilder
	c.ssaValuesInfo = builder.ValuesInfo()

	if diff := len(c.ssaValuesInfo) - len(c.ssaValueToVRegs); diff > 0 {
		c.ssaValueToVRegs = append(c.ssaValueToVRegs, make([]regalloc.VReg, diff+1)...)
	}

	for blk := builder.BlockIteratorReversePostOrderBegin(); blk != nil; blk = builder.BlockIteratorReversePostOrderNext() {
		// First we assign a virtual register to each parameter.
		for i := 0; i < blk.Params(); i++ {
			p := blk.Param(i)
			pid := p.ID()
			typ := p.Type()
			vreg := c.AllocateVReg(typ)
			c.ssaValueToVRegs[pid] = vreg
			c.ssaTypeOfVRegID[vreg.ID()] = p.Type()
		}

		// Assigns each value to a virtual register produced by instructions.
		for cur := blk.Root(); cur != nil; cur = cur.Next() {
			r, rs := cur.Returns()
			if r.Valid() {
				id := r.ID()
				ssaTyp := r.Type()
				typ := r.Type()
				vReg := c.AllocateVReg(typ)
				c.ssaValueToVRegs[id] = vReg
				c.ssaTypeOfVRegID[vReg.ID()] = ssaTyp
			}
			for _, r := range rs {
				id := r.ID()
				ssaTyp := r.Type()
				vReg := c.AllocateVReg(ssaTyp)
				c.ssaValueToVRegs[id] = vReg
				c.ssaTypeOfVRegID[vReg.ID()] = ssaTyp
			}
		}
	}

	for i, retBlk := 0, builder.ReturnBlock(); i < retBlk.Params(); i++ {
		typ := retBlk.Param(i).Type()
		vReg := c.AllocateVReg(typ)
		c.returnVRegs = append(c.returnVRegs, vReg)
		c.ssaTypeOfVRegID[vReg.ID()] = typ
	}
}

// AllocateVReg implements Compiler.AllocateVReg.
func (c *compiler) AllocateVReg(typ ssa.Type) regalloc.VReg {
	regType := regalloc.RegTypeOf(typ)
	r := regalloc.VReg(c.nextVRegID).SetRegType(regType)

	id := r.ID()
	if int(id) >= len(c.ssaTypeOfVRegID) {
		c.ssaTypeOfVRegID = append(c.ssaTypeOfVRegID, make([]ssa.Type, id+1)...)
	}
	c.ssaTypeOfVRegID[id] = typ
	c.nextVRegID++
	return r
}

// Init implements Compiler.Init.
func (c *compiler) Init() {
	c.currentGID = 0
	c.nextVRegID = regalloc.VRegIDNonReservedBegin
	c.returnVRegs = c.returnVRegs[:0]
	c.mach.Reset()
	c.varEdges = c.varEdges[:0]
	c.constEdges = c.constEdges[:0]
	c.buf = c.buf[:0]
	c.sourceOffsets = c.sourceOffsets[:0]
	c.relocations = c.relocations[:0]
}

// ValueDefinition implements Compiler.ValueDefinition.
func (c *compiler) ValueDefinition(value ssa.Value) SSAValueDefinition {
	return SSAValueDefinition{
		V:        value,
		Instr:    c.ssaBuilder.InstructionOfValue(value),
		RefCount: c.ssaValuesInfo[value.ID()].RefCount,
	}
}

// VRegOf implements Compiler.VRegOf.
func (c *compiler) VRegOf(value ssa.Value) regalloc.VReg {
	return c.ssaValueToVRegs[value.ID()]
}

// Format implements Compiler.Format.
func (c *compiler) Format() string {
	return c.mach.Format()
}

// TypeOf implements Compiler.Format.
func (c *compiler) TypeOf(v regalloc.VReg) ssa.Type {
	return c.ssaTypeOfVRegID[v.ID()]
}

// MatchInstr implements Compiler.MatchInstr.
func (c *compiler) MatchInstr(def SSAValueDefinition, opcode ssa.Opcode) bool {
	instr := def.Instr
	return def.IsFromInstr() &&
		instr.Opcode() == opcode &&
		instr.GroupID() == c.currentGID &&
		def.RefCount < 2
}

// MatchInstrOneOf implements Compiler.MatchInstrOneOf.
func (c *compiler) MatchInstrOneOf(def SSAValueDefinition, opcodes []ssa.Opcode) ssa.Opcode {
	instr := def.Instr
	if !def.IsFromInstr() {
		return ssa.OpcodeInvalid
	}

	if instr.GroupID() != c.currentGID {
		return ssa.OpcodeInvalid
	}

	if def.RefCount >= 2 {
		return ssa.OpcodeInvalid
	}

	opcode := instr.Opcode()
	for _, op := range opcodes {
		if opcode == op {
			return opcode
		}
	}
	return ssa.OpcodeInvalid
}

// SSABuilder implements Compiler .SSABuilder.
func (c *compiler) SSABuilder() ssa.Builder {
	return c.ssaBuilder
}

// AddSourceOffsetInfo implements Compiler.AddSourceOffsetInfo.
func (c *compiler) AddSourceOffsetInfo(executableOffset int64, sourceOffset ssa.SourceOffset) {
	c.sourceOffsets = append(c.sourceOffsets, SourceOffsetInfo{
		SourceOffset:     sourceOffset,
		ExecutableOffset: executableOffset,
	})
}

// SourceOffsetInfo implements Compiler.SourceOffsetInfo.
func (c *compiler) SourceOffsetInfo() []SourceOffsetInfo {
	return c.sourceOffsets
}

// AddRelocationInfo implements Compiler.AddRelocationInfo.
func (c *compiler) AddRelocationInfo(funcRef ssa.FuncRef) {
	c.relocations = append(c.relocations, RelocationInfo{
		Offset:  int64(len(c.buf)),
		FuncRef: funcRef,
	})
}

// Emit8Bytes implements Compiler.Emit8Bytes.
func (c *compiler) Emit8Bytes(b uint64) {
	c.buf = append(c.buf, byte(b), byte(b>>8), byte(b>>16), byte(b>>24), byte(b>>32), byte(b>>40), byte(b>>48), byte(b>>56))
}

// Emit4Bytes implements Compiler.Emit4Bytes.
func (c *compiler) Emit4Bytes(b uint32) {
	c.buf = append(c.buf, byte(b), byte(b>>8), byte(b>>16), byte(b>>24))
}

// EmitByte implements Compiler.EmitByte.
func (c *compiler) EmitByte(b byte) {
	c.buf = append(c.buf, b)
}

// Buf implements Compiler.Buf.
func (c *compiler) Buf() []byte {
	return c.buf
}

// BufPtr implements Compiler.BufPtr.
func (c *compiler) BufPtr() *[]byte {
	return &c.buf
}

func (c *compiler) GetFunctionABI(sig *ssa.Signature) *FunctionABI {
	if int(sig.ID) >= len(c.abis) {
		c.abis = append(c.abis, make([]FunctionABI, int(sig.ID)+1)...)
	}

	abi := &c.abis[sig.ID]
	if abi.Initialized {
		return abi
	}

	abi.Init(sig, c.argResultInts, c.argResultFloats)
	return abi
}
