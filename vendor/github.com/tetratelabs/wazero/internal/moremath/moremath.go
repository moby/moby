package moremath

import (
	"math"
)

// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/syntax/values.html#floating-point
const (
	// F32CanonicalNaNBits is the 32-bit float where payload's MSB equals 1 and others are all zero.
	F32CanonicalNaNBits = uint32(0x7fc0_0000)
	// F32CanonicalNaNBitsMask can be used to judge the value `v` is canonical nan as "v&F32CanonicalNaNBitsMask == F32CanonicalNaNBits"
	F32CanonicalNaNBitsMask = uint32(0x7fff_ffff)
	// F64CanonicalNaNBits is the 64-bit float where payload's MSB equals 1 and others are all zero.
	F64CanonicalNaNBits = uint64(0x7ff8_0000_0000_0000)
	// F64CanonicalNaNBitsMask can be used to judge the value `v` is canonical nan as "v&F64CanonicalNaNBitsMask == F64CanonicalNaNBits"
	F64CanonicalNaNBitsMask = uint64(0x7fff_ffff_ffff_ffff)
	// F32ArithmeticNaNPayloadMSB is used to extract the most significant bit of payload of 32-bit arithmetic NaN values
	F32ArithmeticNaNPayloadMSB = uint32(0x0040_0000)
	// F32ExponentMask is used to extract the exponent of 32-bit floating point.
	F32ExponentMask = uint32(0x7f80_0000)
	// F32ArithmeticNaNBits is an example 32-bit arithmetic NaN.
	F32ArithmeticNaNBits = F32CanonicalNaNBits | 0b1 // Set first bit to make this different from the canonical NaN.
	// F64ArithmeticNaNPayloadMSB is used to extract the most significant bit of payload of 64-bit arithmetic NaN values
	F64ArithmeticNaNPayloadMSB = uint64(0x0008_0000_0000_0000)
	// F64ExponentMask is used to extract the exponent of 64-bit floating point.
	F64ExponentMask = uint64(0x7ff0_0000_0000_0000)
	// F64ArithmeticNaNBits is an example 64-bit arithmetic NaN.
	F64ArithmeticNaNBits = F64CanonicalNaNBits | 0b1 // Set first bit to make this different from the canonical NaN.
)

// WasmCompatMin64 is the Wasm spec compatible variant of math.Min for 64-bit floating points.
func WasmCompatMin64(x, y float64) float64 {
	switch {
	case math.IsNaN(x) || math.IsNaN(y):
		return returnF64NaNBinOp(x, y)
	case math.IsInf(x, -1) || math.IsInf(y, -1):
		return math.Inf(-1)
	case x == 0 && x == y:
		if math.Signbit(x) {
			return x
		}
		return y
	}
	if x < y {
		return x
	}
	return y
}

// WasmCompatMin32 is the Wasm spec compatible variant of math.Min for 32-bit floating points.
func WasmCompatMin32(x, y float32) float32 {
	x64, y64 := float64(x), float64(y)
	switch {
	case math.IsNaN(x64) || math.IsNaN(y64):
		return returnF32NaNBinOp(x, y)
	case math.IsInf(x64, -1) || math.IsInf(y64, -1):
		return float32(math.Inf(-1))
	case x == 0 && x == y:
		if math.Signbit(x64) {
			return x
		}
		return y
	}
	if x < y {
		return x
	}
	return y
}

// WasmCompatMax64 is the Wasm spec compatible variant of math.Max for 64-bit floating points.
func WasmCompatMax64(x, y float64) float64 {
	switch {
	case math.IsNaN(x) || math.IsNaN(y):
		return returnF64NaNBinOp(x, y)
	case math.IsInf(x, 1) || math.IsInf(y, 1):
		return math.Inf(1)
	case x == 0 && x == y:
		if math.Signbit(x) {
			return y
		}
		return x
	}
	if x > y {
		return x
	}
	return y
}

// WasmCompatMax32 is the Wasm spec compatible variant of math.Max for 32-bit floating points.
func WasmCompatMax32(x, y float32) float32 {
	x64, y64 := float64(x), float64(y)
	switch {
	case math.IsNaN(x64) || math.IsNaN(y64):
		return returnF32NaNBinOp(x, y)
	case math.IsInf(x64, 1) || math.IsInf(y64, 1):
		return float32(math.Inf(1))
	case x == 0 && x == y:
		if math.Signbit(x64) {
			return y
		}
		return x
	}
	if x > y {
		return x
	}
	return y
}

// WasmCompatNearestF32 is the Wasm spec compatible variant of math.Round, used for Nearest instruction.
// For example, this converts 1.9 to 2.0, and this has the semantics of LLVM's rint intrinsic.
//
// e.g. math.Round(-4.5) results in -5 while this results in -4.
//
// See https://llvm.org/docs/LangRef.html#llvm-rint-intrinsic.
func WasmCompatNearestF32(f float32) float32 {
	var res float32
	// TODO: look at https://github.com/bytecodealliance/wasmtime/pull/2171 and reconsider this algorithm
	if f != 0 {
		ceil := float32(math.Ceil(float64(f)))
		floor := float32(math.Floor(float64(f)))
		distToCeil := math.Abs(float64(f - ceil))
		distToFloor := math.Abs(float64(f - floor))
		h := ceil / 2.0
		if distToCeil < distToFloor {
			res = ceil
		} else if distToCeil == distToFloor && float32(math.Floor(float64(h))) == h {
			res = ceil
		} else {
			res = floor
		}
	} else {
		res = f
	}
	return returnF32UniOp(f, res)
}

// WasmCompatNearestF64 is the Wasm spec compatible variant of math.Round, used for Nearest instruction.
// For example, this converts 1.9 to 2.0, and this has the semantics of LLVM's rint intrinsic.
//
// e.g. math.Round(-4.5) results in -5 while this results in -4.
//
// See https://llvm.org/docs/LangRef.html#llvm-rint-intrinsic.
func WasmCompatNearestF64(f float64) float64 {
	// TODO: look at https://github.com/bytecodealliance/wasmtime/pull/2171 and reconsider this algorithm
	var res float64
	if f != 0 {
		ceil := math.Ceil(f)
		floor := math.Floor(f)
		distToCeil := math.Abs(f - ceil)
		distToFloor := math.Abs(f - floor)
		h := ceil / 2.0
		if distToCeil < distToFloor {
			res = ceil
		} else if distToCeil == distToFloor && math.Floor(h) == h {
			res = ceil
		} else {
			res = floor
		}
	} else {
		res = f
	}
	return returnF64UniOp(f, res)
}

// WasmCompatCeilF32 is the same as math.Ceil on 32-bit except that
// the returned NaN value follows the Wasm specification on NaN
// propagation.
// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/numerics.html#nan-propagation
func WasmCompatCeilF32(f float32) float32 {
	return returnF32UniOp(f, float32(math.Ceil(float64(f))))
}

// WasmCompatCeilF64 is the same as math.Ceil on 64-bit except that
// the returned NaN value follows the Wasm specification on NaN
// propagation.
// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/numerics.html#nan-propagation
func WasmCompatCeilF64(f float64) float64 {
	return returnF64UniOp(f, math.Ceil(f))
}

// WasmCompatFloorF32 is the same as math.Floor on 32-bit except that
// the returned NaN value follows the Wasm specification on NaN
// propagation.
// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/numerics.html#nan-propagation
func WasmCompatFloorF32(f float32) float32 {
	return returnF32UniOp(f, float32(math.Floor(float64(f))))
}

// WasmCompatFloorF64 is the same as math.Floor on 64-bit except that
// the returned NaN value follows the Wasm specification on NaN
// propagation.
// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/numerics.html#nan-propagation
func WasmCompatFloorF64(f float64) float64 {
	return returnF64UniOp(f, math.Floor(f))
}

// WasmCompatTruncF32 is the same as math.Trunc on 32-bit except that
// the returned NaN value follows the Wasm specification on NaN
// propagation.
// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/numerics.html#nan-propagation
func WasmCompatTruncF32(f float32) float32 {
	return returnF32UniOp(f, float32(math.Trunc(float64(f))))
}

// WasmCompatTruncF64 is the same as math.Trunc on 64-bit except that
// the returned NaN value follows the Wasm specification on NaN
// propagation.
// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/numerics.html#nan-propagation
func WasmCompatTruncF64(f float64) float64 {
	return returnF64UniOp(f, math.Trunc(f))
}

func f32IsNaN(v float32) bool {
	return v != v // this is how NaN is defined.
}

func f64IsNaN(v float64) bool {
	return v != v // this is how NaN is defined.
}

// returnF32UniOp returns the result of 32-bit unary operation. This accepts `original` which is the operand,
// and `result` which is its result. This returns the `result` as-is if the result is not NaN. Otherwise, this follows
// the same logic as in the reference interpreter as well as the amd64 and arm64 floating point handling.
func returnF32UniOp(original, result float32) float32 {
	// Following the same logic as in the reference interpreter:
	// https://github.com/WebAssembly/spec/blob/d48af683f5e6d00c13f775ab07d29a15daf92203/interpreter/exec/fxx.ml#L115-L122
	if !f32IsNaN(result) {
		return result
	}
	if !f32IsNaN(original) {
		return math.Float32frombits(F32CanonicalNaNBits)
	}
	return math.Float32frombits(math.Float32bits(original) | F32CanonicalNaNBits)
}

// returnF32UniOp returns the result of 64-bit unary operation. This accepts `original` which is the operand,
// and `result` which is its result. This returns the `result` as-is if the result is not NaN. Otherwise, this follows
// the same logic as in the reference interpreter as well as the amd64 and arm64 floating point handling.
func returnF64UniOp(original, result float64) float64 {
	// Following the same logic as in the reference interpreter (== amd64 and arm64's behavior):
	// https://github.com/WebAssembly/spec/blob/d48af683f5e6d00c13f775ab07d29a15daf92203/interpreter/exec/fxx.ml#L115-L122
	if !f64IsNaN(result) {
		return result
	}
	if !f64IsNaN(original) {
		return math.Float64frombits(F64CanonicalNaNBits)
	}
	return math.Float64frombits(math.Float64bits(original) | F64CanonicalNaNBits)
}

// returnF64NaNBinOp returns a NaN for 64-bit binary operations. `x` and `y` are original floats
// and at least one of them is NaN. The returned NaN is guaranteed to comply with the NaN propagation
// procedure: https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/numerics.html#nan-propagation
func returnF64NaNBinOp(x, y float64) float64 {
	if f64IsNaN(x) {
		return math.Float64frombits(math.Float64bits(x) | F64CanonicalNaNBits)
	} else {
		return math.Float64frombits(math.Float64bits(y) | F64CanonicalNaNBits)
	}
}

// returnF64NaNBinOp returns a NaN for 32-bit binary operations. `x` and `y` are original floats
// and at least one of them is NaN. The returned NaN is guaranteed to comply with the NaN propagation
// procedure: https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/numerics.html#nan-propagation
func returnF32NaNBinOp(x, y float32) float32 {
	if f32IsNaN(x) {
		return math.Float32frombits(math.Float32bits(x) | F32CanonicalNaNBits)
	} else {
		return math.Float32frombits(math.Float32bits(y) | F32CanonicalNaNBits)
	}
}
