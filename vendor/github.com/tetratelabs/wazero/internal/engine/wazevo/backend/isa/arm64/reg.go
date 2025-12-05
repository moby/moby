package arm64

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
)

// Arm64-specific registers.
//
// See https://developer.arm.com/documentation/dui0801/a/Overview-of-AArch64-state/Predeclared-core-register-names-in-AArch64-state

const (
	// General purpose registers. Note that we do not distinguish wn and xn registers
	// because they are the same from the perspective of register allocator, and
	// the size can be determined by the type of the instruction.

	x0 = regalloc.RealRegInvalid + 1 + iota
	x1
	x2
	x3
	x4
	x5
	x6
	x7
	x8
	x9
	x10
	x11
	x12
	x13
	x14
	x15
	x16
	x17
	x18
	x19
	x20
	x21
	x22
	x23
	x24
	x25
	x26
	x27
	x28
	x29
	x30

	// Vector registers. Note that we do not distinguish vn and dn, ... registers
	// because they are the same from the perspective of register allocator, and
	// the size can be determined by the type of the instruction.

	v0
	v1
	v2
	v3
	v4
	v5
	v6
	v7
	v8
	v9
	v10
	v11
	v12
	v13
	v14
	v15
	v16
	v17
	v18
	v19
	v20
	v21
	v22
	v23
	v24
	v25
	v26
	v27
	v28
	v29
	v30
	v31

	// Special registers

	xzr
	sp
	lr  = x30
	fp  = x29
	tmp = x27
)

var (
	x0VReg  = regalloc.FromRealReg(x0, regalloc.RegTypeInt)
	x1VReg  = regalloc.FromRealReg(x1, regalloc.RegTypeInt)
	x2VReg  = regalloc.FromRealReg(x2, regalloc.RegTypeInt)
	x3VReg  = regalloc.FromRealReg(x3, regalloc.RegTypeInt)
	x4VReg  = regalloc.FromRealReg(x4, regalloc.RegTypeInt)
	x5VReg  = regalloc.FromRealReg(x5, regalloc.RegTypeInt)
	x6VReg  = regalloc.FromRealReg(x6, regalloc.RegTypeInt)
	x7VReg  = regalloc.FromRealReg(x7, regalloc.RegTypeInt)
	x8VReg  = regalloc.FromRealReg(x8, regalloc.RegTypeInt)
	x9VReg  = regalloc.FromRealReg(x9, regalloc.RegTypeInt)
	x10VReg = regalloc.FromRealReg(x10, regalloc.RegTypeInt)
	x11VReg = regalloc.FromRealReg(x11, regalloc.RegTypeInt)
	x12VReg = regalloc.FromRealReg(x12, regalloc.RegTypeInt)
	x13VReg = regalloc.FromRealReg(x13, regalloc.RegTypeInt)
	x14VReg = regalloc.FromRealReg(x14, regalloc.RegTypeInt)
	x15VReg = regalloc.FromRealReg(x15, regalloc.RegTypeInt)
	x16VReg = regalloc.FromRealReg(x16, regalloc.RegTypeInt)
	x17VReg = regalloc.FromRealReg(x17, regalloc.RegTypeInt)
	x18VReg = regalloc.FromRealReg(x18, regalloc.RegTypeInt)
	x19VReg = regalloc.FromRealReg(x19, regalloc.RegTypeInt)
	x20VReg = regalloc.FromRealReg(x20, regalloc.RegTypeInt)
	x21VReg = regalloc.FromRealReg(x21, regalloc.RegTypeInt)
	x22VReg = regalloc.FromRealReg(x22, regalloc.RegTypeInt)
	x23VReg = regalloc.FromRealReg(x23, regalloc.RegTypeInt)
	x24VReg = regalloc.FromRealReg(x24, regalloc.RegTypeInt)
	x25VReg = regalloc.FromRealReg(x25, regalloc.RegTypeInt)
	x26VReg = regalloc.FromRealReg(x26, regalloc.RegTypeInt)
	x27VReg = regalloc.FromRealReg(x27, regalloc.RegTypeInt)
	x28VReg = regalloc.FromRealReg(x28, regalloc.RegTypeInt)
	x29VReg = regalloc.FromRealReg(x29, regalloc.RegTypeInt)
	x30VReg = regalloc.FromRealReg(x30, regalloc.RegTypeInt)
	v0VReg  = regalloc.FromRealReg(v0, regalloc.RegTypeFloat)
	v1VReg  = regalloc.FromRealReg(v1, regalloc.RegTypeFloat)
	v2VReg  = regalloc.FromRealReg(v2, regalloc.RegTypeFloat)
	v3VReg  = regalloc.FromRealReg(v3, regalloc.RegTypeFloat)
	v4VReg  = regalloc.FromRealReg(v4, regalloc.RegTypeFloat)
	v5VReg  = regalloc.FromRealReg(v5, regalloc.RegTypeFloat)
	v6VReg  = regalloc.FromRealReg(v6, regalloc.RegTypeFloat)
	v7VReg  = regalloc.FromRealReg(v7, regalloc.RegTypeFloat)
	v8VReg  = regalloc.FromRealReg(v8, regalloc.RegTypeFloat)
	v9VReg  = regalloc.FromRealReg(v9, regalloc.RegTypeFloat)
	v10VReg = regalloc.FromRealReg(v10, regalloc.RegTypeFloat)
	v11VReg = regalloc.FromRealReg(v11, regalloc.RegTypeFloat)
	v12VReg = regalloc.FromRealReg(v12, regalloc.RegTypeFloat)
	v13VReg = regalloc.FromRealReg(v13, regalloc.RegTypeFloat)
	v14VReg = regalloc.FromRealReg(v14, regalloc.RegTypeFloat)
	v15VReg = regalloc.FromRealReg(v15, regalloc.RegTypeFloat)
	v16VReg = regalloc.FromRealReg(v16, regalloc.RegTypeFloat)
	v17VReg = regalloc.FromRealReg(v17, regalloc.RegTypeFloat)
	v18VReg = regalloc.FromRealReg(v18, regalloc.RegTypeFloat)
	v19VReg = regalloc.FromRealReg(v19, regalloc.RegTypeFloat)
	v20VReg = regalloc.FromRealReg(v20, regalloc.RegTypeFloat)
	v21VReg = regalloc.FromRealReg(v21, regalloc.RegTypeFloat)
	v22VReg = regalloc.FromRealReg(v22, regalloc.RegTypeFloat)
	v23VReg = regalloc.FromRealReg(v23, regalloc.RegTypeFloat)
	v24VReg = regalloc.FromRealReg(v24, regalloc.RegTypeFloat)
	v25VReg = regalloc.FromRealReg(v25, regalloc.RegTypeFloat)
	v26VReg = regalloc.FromRealReg(v26, regalloc.RegTypeFloat)
	v27VReg = regalloc.FromRealReg(v27, regalloc.RegTypeFloat)
	// lr (link register) holds the return address at the function entry.
	lrVReg = x30VReg
	// tmpReg is used to perform spill/load on large stack offsets, and load large constants.
	// Therefore, be cautious to use this register in the middle of the compilation, especially before the register allocation.
	// This is the same as golang/go, but it's only described in the source code:
	// https://github.com/golang/go/blob/18e17e2cb12837ea2c8582ecdb0cc780f49a1aac/src/cmd/compile/internal/ssa/_gen/ARM64Ops.go#L59
	// https://github.com/golang/go/blob/18e17e2cb12837ea2c8582ecdb0cc780f49a1aac/src/cmd/compile/internal/ssa/_gen/ARM64Ops.go#L13-L15
	tmpRegVReg = regalloc.FromRealReg(tmp, regalloc.RegTypeInt)
	v28VReg    = regalloc.FromRealReg(v28, regalloc.RegTypeFloat)
	v29VReg    = regalloc.FromRealReg(v29, regalloc.RegTypeFloat)
	v30VReg    = regalloc.FromRealReg(v30, regalloc.RegTypeFloat)
	v31VReg    = regalloc.FromRealReg(v31, regalloc.RegTypeFloat)
	xzrVReg    = regalloc.FromRealReg(xzr, regalloc.RegTypeInt)
	spVReg     = regalloc.FromRealReg(sp, regalloc.RegTypeInt)
	fpVReg     = regalloc.FromRealReg(fp, regalloc.RegTypeInt)
)

var regNames = [...]string{
	x0:  "x0",
	x1:  "x1",
	x2:  "x2",
	x3:  "x3",
	x4:  "x4",
	x5:  "x5",
	x6:  "x6",
	x7:  "x7",
	x8:  "x8",
	x9:  "x9",
	x10: "x10",
	x11: "x11",
	x12: "x12",
	x13: "x13",
	x14: "x14",
	x15: "x15",
	x16: "x16",
	x17: "x17",
	x18: "x18",
	x19: "x19",
	x20: "x20",
	x21: "x21",
	x22: "x22",
	x23: "x23",
	x24: "x24",
	x25: "x25",
	x26: "x26",
	x27: "x27",
	x28: "x28",
	x29: "x29",
	x30: "x30",
	xzr: "xzr",
	sp:  "sp",
	v0:  "v0",
	v1:  "v1",
	v2:  "v2",
	v3:  "v3",
	v4:  "v4",
	v5:  "v5",
	v6:  "v6",
	v7:  "v7",
	v8:  "v8",
	v9:  "v9",
	v10: "v10",
	v11: "v11",
	v12: "v12",
	v13: "v13",
	v14: "v14",
	v15: "v15",
	v16: "v16",
	v17: "v17",
	v18: "v18",
	v19: "v19",
	v20: "v20",
	v21: "v21",
	v22: "v22",
	v23: "v23",
	v24: "v24",
	v25: "v25",
	v26: "v26",
	v27: "v27",
	v28: "v28",
	v29: "v29",
	v30: "v30",
	v31: "v31",
}

func formatVRegSized(r regalloc.VReg, size byte) (ret string) {
	if r.IsRealReg() {
		ret = regNames[r.RealReg()]
		switch ret[0] {
		case 'x':
			switch size {
			case 32:
				ret = strings.Replace(ret, "x", "w", 1)
			case 64:
			default:
				panic("BUG: invalid register size: " + strconv.Itoa(int(size)))
			}
		case 'v':
			switch size {
			case 32:
				ret = strings.Replace(ret, "v", "s", 1)
			case 64:
				ret = strings.Replace(ret, "v", "d", 1)
			case 128:
				ret = strings.Replace(ret, "v", "q", 1)
			default:
				panic("BUG: invalid register size")
			}
		}
	} else {
		switch r.RegType() {
		case regalloc.RegTypeInt:
			switch size {
			case 32:
				ret = fmt.Sprintf("w%d?", r.ID())
			case 64:
				ret = fmt.Sprintf("x%d?", r.ID())
			default:
				panic("BUG: invalid register size: " + strconv.Itoa(int(size)))
			}
		case regalloc.RegTypeFloat:
			switch size {
			case 32:
				ret = fmt.Sprintf("s%d?", r.ID())
			case 64:
				ret = fmt.Sprintf("d%d?", r.ID())
			case 128:
				ret = fmt.Sprintf("q%d?", r.ID())
			default:
				panic("BUG: invalid register size")
			}
		default:
			panic(fmt.Sprintf("BUG: invalid register type: %d for %s", r.RegType(), r))
		}
	}
	return
}

func formatVRegWidthVec(r regalloc.VReg, width vecArrangement) (ret string) {
	var id string
	wspec := strings.ToLower(width.String())
	if r.IsRealReg() {
		id = regNames[r.RealReg()][1:]
	} else {
		id = fmt.Sprintf("%d?", r.ID())
	}
	ret = fmt.Sprintf("%s%s", wspec, id)
	return
}

func formatVRegVec(r regalloc.VReg, arr vecArrangement, index vecIndex) (ret string) {
	id := fmt.Sprintf("v%d?", r.ID())
	if r.IsRealReg() {
		id = regNames[r.RealReg()]
	}
	ret = fmt.Sprintf("%s.%s", id, strings.ToLower(arr.String()))
	if index != vecIndexNone {
		ret += fmt.Sprintf("[%d]", index)
	}
	return
}

func regTypeToRegisterSizeInBits(r regalloc.RegType) byte {
	switch r {
	case regalloc.RegTypeInt:
		return 64
	case regalloc.RegTypeFloat:
		return 128
	default:
		panic("BUG: invalid register type")
	}
}

var regNumberInEncoding = [...]uint32{
	x0:  0,
	x1:  1,
	x2:  2,
	x3:  3,
	x4:  4,
	x5:  5,
	x6:  6,
	x7:  7,
	x8:  8,
	x9:  9,
	x10: 10,
	x11: 11,
	x12: 12,
	x13: 13,
	x14: 14,
	x15: 15,
	x16: 16,
	x17: 17,
	x18: 18,
	x19: 19,
	x20: 20,
	x21: 21,
	x22: 22,
	x23: 23,
	x24: 24,
	x25: 25,
	x26: 26,
	x27: 27,
	x28: 28,
	x29: 29,
	x30: 30,
	xzr: 31,
	sp:  31,
	v0:  0,
	v1:  1,
	v2:  2,
	v3:  3,
	v4:  4,
	v5:  5,
	v6:  6,
	v7:  7,
	v8:  8,
	v9:  9,
	v10: 10,
	v11: 11,
	v12: 12,
	v13: 13,
	v14: 14,
	v15: 15,
	v16: 16,
	v17: 17,
	v18: 18,
	v19: 19,
	v20: 20,
	v21: 21,
	v22: 22,
	v23: 23,
	v24: 24,
	v25: 25,
	v26: 26,
	v27: 27,
	v28: 28,
	v29: 29,
	v30: 30,
	v31: 31,
}
