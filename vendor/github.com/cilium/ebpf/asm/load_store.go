package asm

import "fmt"

//go:generate go tool stringer -output load_store_string.go -type=Mode,Size

// Mode for load and store operations
//
//	msb      lsb
//	+---+--+---+
//	|MDE|sz|cls|
//	+---+--+---+
type Mode uint8

const modeMask OpCode = 0xe0

const (
	// InvalidMode is returned by getters when invoked
	// on non load / store OpCodes
	InvalidMode Mode = 0xff
	// ImmMode - immediate value
	ImmMode Mode = 0x00
	// AbsMode - immediate value + offset
	AbsMode Mode = 0x20
	// IndMode - indirect (imm+src)
	IndMode Mode = 0x40
	// MemMode - load from memory
	MemMode Mode = 0x60
	// MemSXMode - load from memory, sign extension
	MemSXMode Mode = 0x80
	// AtomicMode - add atomically across processors.
	AtomicMode Mode = 0xc0
)

const atomicMask OpCode = 0x0001_ff00

type AtomicOp uint32

const (
	InvalidAtomic AtomicOp = 0xffff_ffff

	// AddAtomic - add src to memory address dst atomically
	AddAtomic AtomicOp = AtomicOp(Add) << 8
	// FetchAdd - add src to memory address dst atomically, store result in src
	FetchAdd AtomicOp = AddAtomic | fetch
	// AndAtomic - bitwise AND src with memory address at dst atomically
	AndAtomic AtomicOp = AtomicOp(And) << 8
	// FetchAnd - bitwise AND src with memory address at dst atomically, store result in src
	FetchAnd AtomicOp = AndAtomic | fetch
	// OrAtomic - bitwise OR src with memory address at dst atomically
	OrAtomic AtomicOp = AtomicOp(Or) << 8
	// FetchOr - bitwise OR src with memory address at dst atomically, store result in src
	FetchOr AtomicOp = OrAtomic | fetch
	// XorAtomic - bitwise XOR src with memory address at dst atomically
	XorAtomic AtomicOp = AtomicOp(Xor) << 8
	// FetchXor - bitwise XOR src with memory address at dst atomically, store result in src
	FetchXor AtomicOp = XorAtomic | fetch

	// Xchg - atomically exchange the old value with the new value
	//
	// src gets populated with the old value of *(size *)(dst + offset).
	Xchg AtomicOp = 0x0000_e000 | fetch
	// CmpXchg - atomically compare and exchange the old value with the new value
	//
	// Compares R0 and *(size *)(dst + offset), writes src to *(size *)(dst + offset) on match.
	// R0 gets populated with the old value of *(size *)(dst + offset), even if no exchange occurs.
	CmpXchg AtomicOp = 0x0000_f000 | fetch

	// fetch modifier for copy-modify-write atomics
	fetch AtomicOp = 0x0000_0100
	// loadAcquire - atomically load with acquire semantics
	loadAcquire AtomicOp = 0x0001_0000
	// storeRelease - atomically store with release semantics
	storeRelease AtomicOp = 0x0001_1000
)

func (op AtomicOp) String() string {
	var name string
	switch op {
	case AddAtomic, AndAtomic, OrAtomic, XorAtomic:
		name = ALUOp(op >> 8).String()
	case FetchAdd, FetchAnd, FetchOr, FetchXor:
		name = "Fetch" + ALUOp((op^fetch)>>8).String()
	case Xchg:
		name = "Xchg"
	case CmpXchg:
		name = "CmpXchg"
	case loadAcquire:
		name = "LdAcq"
	case storeRelease:
		name = "StRel"
	default:
		name = fmt.Sprintf("AtomicOp(%#x)", uint32(op))
	}

	return name
}

func (op AtomicOp) OpCode(size Size) OpCode {
	switch op {
	case AddAtomic, AndAtomic, OrAtomic, XorAtomic,
		FetchAdd, FetchAnd, FetchOr, FetchXor,
		Xchg, CmpXchg:
		switch size {
		case Byte, Half:
			// 8-bit and 16-bit atomic copy-modify-write atomics are not supported
			return InvalidOpCode
		}
	}

	return OpCode(StXClass).SetMode(AtomicMode).SetSize(size).SetAtomicOp(op)
}

// Mem emits `*(size *)(dst + offset) (op) src`.
func (op AtomicOp) Mem(dst, src Register, size Size, offset int16) Instruction {
	return Instruction{
		OpCode: op.OpCode(size),
		Dst:    dst,
		Src:    src,
		Offset: offset,
	}
}

// Emits `lock-acquire dst = *(size *)(src + offset)`.
func LoadAcquire(dst, src Register, size Size, offset int16) Instruction {
	return Instruction{
		OpCode: loadAcquire.OpCode(size),
		Dst:    dst,
		Src:    src,
		Offset: offset,
	}
}

// Emits `lock-release *(size *)(dst + offset) = src`.
func StoreRelease(dst, src Register, size Size, offset int16) Instruction {
	return Instruction{
		OpCode: storeRelease.OpCode(size),
		Dst:    dst,
		Src:    src,
		Offset: offset,
	}
}

// Size of load and store operations
//
//	msb      lsb
//	+---+--+---+
//	|mde|SZ|cls|
//	+---+--+---+
type Size uint8

const sizeMask OpCode = 0x18

const (
	// InvalidSize is returned by getters when invoked
	// on non load / store OpCodes
	InvalidSize Size = 0xff
	// DWord - double word; 64 bits
	DWord Size = 0x18
	// Word - word; 32 bits
	Word Size = 0x00
	// Half - half-word; 16 bits
	Half Size = 0x08
	// Byte - byte; 8 bits
	Byte Size = 0x10
)

// Sizeof returns the size in bytes.
func (s Size) Sizeof() int {
	switch s {
	case DWord:
		return 8
	case Word:
		return 4
	case Half:
		return 2
	case Byte:
		return 1
	default:
		return -1
	}
}

// LoadMemOp returns the OpCode to load a value of given size from memory.
func LoadMemOp(size Size) OpCode {
	return OpCode(LdXClass).SetMode(MemMode).SetSize(size)
}

// LoadMemSXOp returns the OpCode to load a value of given size from memory sign extended.
func LoadMemSXOp(size Size) OpCode {
	return OpCode(LdXClass).SetMode(MemSXMode).SetSize(size)
}

// LoadMem emits `dst = *(size *)(src + offset)`.
func LoadMem(dst, src Register, offset int16, size Size) Instruction {
	return Instruction{
		OpCode: LoadMemOp(size),
		Dst:    dst,
		Src:    src,
		Offset: offset,
	}
}

// LoadMemSX emits `dst = *(size *)(src + offset)` but sign extends dst.
func LoadMemSX(dst, src Register, offset int16, size Size) Instruction {
	if size == DWord {
		return Instruction{OpCode: InvalidOpCode}
	}

	return Instruction{
		OpCode: LoadMemSXOp(size),
		Dst:    dst,
		Src:    src,
		Offset: offset,
	}
}

// LoadImmOp returns the OpCode to load an immediate of given size.
//
// As of kernel 4.20, only DWord size is accepted.
func LoadImmOp(size Size) OpCode {
	return OpCode(LdClass).SetMode(ImmMode).SetSize(size)
}

// LoadImm emits `dst = (size)value`.
//
// As of kernel 4.20, only DWord size is accepted.
func LoadImm(dst Register, value int64, size Size) Instruction {
	return Instruction{
		OpCode:   LoadImmOp(size),
		Dst:      dst,
		Constant: value,
	}
}

// LoadMapPtr stores a pointer to a map in dst.
func LoadMapPtr(dst Register, fd int) Instruction {
	if fd < 0 {
		return Instruction{OpCode: InvalidOpCode}
	}

	return Instruction{
		OpCode:   LoadImmOp(DWord),
		Dst:      dst,
		Src:      PseudoMapFD,
		Constant: int64(uint32(fd)),
	}
}

// LoadMapValue stores a pointer to the value at a certain offset of a map.
func LoadMapValue(dst Register, fd int, offset uint32) Instruction {
	if fd < 0 {
		return Instruction{OpCode: InvalidOpCode}
	}

	fdAndOffset := (uint64(offset) << 32) | uint64(uint32(fd))
	return Instruction{
		OpCode:   LoadImmOp(DWord),
		Dst:      dst,
		Src:      PseudoMapValue,
		Constant: int64(fdAndOffset),
	}
}

// LoadIndOp returns the OpCode for loading a value of given size from an sk_buff.
func LoadIndOp(size Size) OpCode {
	return OpCode(LdClass).SetMode(IndMode).SetSize(size)
}

// LoadInd emits `dst = ntoh(*(size *)(((sk_buff *)R6)->data + src + offset))`.
func LoadInd(dst, src Register, offset int32, size Size) Instruction {
	return Instruction{
		OpCode:   LoadIndOp(size),
		Dst:      dst,
		Src:      src,
		Constant: int64(offset),
	}
}

// LoadAbsOp returns the OpCode for loading a value of given size from an sk_buff.
func LoadAbsOp(size Size) OpCode {
	return OpCode(LdClass).SetMode(AbsMode).SetSize(size)
}

// LoadAbs emits `r0 = ntoh(*(size *)(((sk_buff *)R6)->data + offset))`.
func LoadAbs(offset int32, size Size) Instruction {
	return Instruction{
		OpCode:   LoadAbsOp(size),
		Dst:      R0,
		Constant: int64(offset),
	}
}

// StoreMemOp returns the OpCode for storing a register of given size in memory.
func StoreMemOp(size Size) OpCode {
	return OpCode(StXClass).SetMode(MemMode).SetSize(size)
}

// StoreMem emits `*(size *)(dst + offset) = src`
func StoreMem(dst Register, offset int16, src Register, size Size) Instruction {
	return Instruction{
		OpCode: StoreMemOp(size),
		Dst:    dst,
		Src:    src,
		Offset: offset,
	}
}

// StoreImmOp returns the OpCode for storing an immediate of given size in memory.
func StoreImmOp(size Size) OpCode {
	return OpCode(StClass).SetMode(MemMode).SetSize(size)
}

// StoreImm emits `*(size *)(dst + offset) = value`.
func StoreImm(dst Register, offset int16, value int64, size Size) Instruction {
	if size == DWord {
		return Instruction{OpCode: InvalidOpCode}
	}

	return Instruction{
		OpCode:   StoreImmOp(size),
		Dst:      dst,
		Offset:   offset,
		Constant: value,
	}
}

// StoreXAddOp returns the OpCode to atomically add a register to a value in memory.
func StoreXAddOp(size Size) OpCode {
	return AddAtomic.OpCode(size)
}

// StoreXAdd atomically adds src to *dst.
func StoreXAdd(dst, src Register, size Size) Instruction {
	return AddAtomic.Mem(dst, src, size, 0)
}
