package asm

import "github.com/cilium/ebpf/internal/platform"

//go:generate go tool stringer -output func_string.go -type=BuiltinFunc

// BuiltinFunc is a built-in eBPF function.
type BuiltinFunc uint32

// BuiltinFuncForPlatform returns a platform specific function constant.
//
// Use this if the library doesn't provide a constant yet.
func BuiltinFuncForPlatform(plat string, value uint32) (BuiltinFunc, error) {
	return platform.EncodeConstant[BuiltinFunc](plat, value)
}

// Call emits a function call.
func (fn BuiltinFunc) Call() Instruction {
	return Instruction{
		OpCode:   OpCode(JumpClass).SetJumpOp(Call),
		Constant: int64(fn),
	}
}
