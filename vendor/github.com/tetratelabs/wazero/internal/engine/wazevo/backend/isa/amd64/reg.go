package amd64

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
)

// Amd64-specific registers.
const (
	// rax is a gp register.
	rax = regalloc.RealRegInvalid + 1 + iota
	// rcx is a gp register.
	rcx
	// rdx is a gp register.
	rdx
	// rbx is a gp register.
	rbx
	// rsp is a gp register.
	rsp
	// rbp is a gp register.
	rbp
	// rsi is a gp register.
	rsi
	// rdi is a gp register.
	rdi
	// r8 is a gp register.
	r8
	// r9 is a gp register.
	r9
	// r10 is a gp register.
	r10
	// r11 is a gp register.
	r11
	// r12 is a gp register.
	r12
	// r13 is a gp register.
	r13
	// r14 is a gp register.
	r14
	// r15 is a gp register.
	r15

	// xmm0 is a vector register.
	xmm0
	// xmm1 is a vector register.
	xmm1
	// xmm2 is a vector register.
	xmm2
	// xmm3 is a vector register.
	xmm3
	// xmm4 is a vector register.
	xmm4
	// xmm5 is a vector register.
	xmm5
	// xmm6 is a vector register.
	xmm6
	// xmm7 is a vector register.
	xmm7
	// xmm8 is a vector register.
	xmm8
	// xmm9 is a vector register.
	xmm9
	// xmm10 is a vector register.
	xmm10
	// xmm11 is a vector register.
	xmm11
	// xmm12 is a vector register.
	xmm12
	// xmm13 is a vector register.
	xmm13
	// xmm14 is a vector register.
	xmm14
	// xmm15 is a vector register.
	xmm15
)

var (
	raxVReg = regalloc.FromRealReg(rax, regalloc.RegTypeInt)
	rcxVReg = regalloc.FromRealReg(rcx, regalloc.RegTypeInt)
	rdxVReg = regalloc.FromRealReg(rdx, regalloc.RegTypeInt)
	rbxVReg = regalloc.FromRealReg(rbx, regalloc.RegTypeInt)
	rspVReg = regalloc.FromRealReg(rsp, regalloc.RegTypeInt)
	rbpVReg = regalloc.FromRealReg(rbp, regalloc.RegTypeInt)
	rsiVReg = regalloc.FromRealReg(rsi, regalloc.RegTypeInt)
	rdiVReg = regalloc.FromRealReg(rdi, regalloc.RegTypeInt)
	r8VReg  = regalloc.FromRealReg(r8, regalloc.RegTypeInt)
	r9VReg  = regalloc.FromRealReg(r9, regalloc.RegTypeInt)
	r10VReg = regalloc.FromRealReg(r10, regalloc.RegTypeInt)
	r11VReg = regalloc.FromRealReg(r11, regalloc.RegTypeInt)
	r12VReg = regalloc.FromRealReg(r12, regalloc.RegTypeInt)
	r13VReg = regalloc.FromRealReg(r13, regalloc.RegTypeInt)
	r14VReg = regalloc.FromRealReg(r14, regalloc.RegTypeInt)
	r15VReg = regalloc.FromRealReg(r15, regalloc.RegTypeInt)

	xmm0VReg  = regalloc.FromRealReg(xmm0, regalloc.RegTypeFloat)
	xmm1VReg  = regalloc.FromRealReg(xmm1, regalloc.RegTypeFloat)
	xmm2VReg  = regalloc.FromRealReg(xmm2, regalloc.RegTypeFloat)
	xmm3VReg  = regalloc.FromRealReg(xmm3, regalloc.RegTypeFloat)
	xmm4VReg  = regalloc.FromRealReg(xmm4, regalloc.RegTypeFloat)
	xmm5VReg  = regalloc.FromRealReg(xmm5, regalloc.RegTypeFloat)
	xmm6VReg  = regalloc.FromRealReg(xmm6, regalloc.RegTypeFloat)
	xmm7VReg  = regalloc.FromRealReg(xmm7, regalloc.RegTypeFloat)
	xmm8VReg  = regalloc.FromRealReg(xmm8, regalloc.RegTypeFloat)
	xmm9VReg  = regalloc.FromRealReg(xmm9, regalloc.RegTypeFloat)
	xmm10VReg = regalloc.FromRealReg(xmm10, regalloc.RegTypeFloat)
	xmm11VReg = regalloc.FromRealReg(xmm11, regalloc.RegTypeFloat)
	xmm12VReg = regalloc.FromRealReg(xmm12, regalloc.RegTypeFloat)
	xmm13VReg = regalloc.FromRealReg(xmm13, regalloc.RegTypeFloat)
	xmm14VReg = regalloc.FromRealReg(xmm14, regalloc.RegTypeFloat)
	xmm15VReg = regalloc.FromRealReg(xmm15, regalloc.RegTypeFloat)
)

var regNames = [...]string{
	rax:   "rax",
	rcx:   "rcx",
	rdx:   "rdx",
	rbx:   "rbx",
	rsp:   "rsp",
	rbp:   "rbp",
	rsi:   "rsi",
	rdi:   "rdi",
	r8:    "r8",
	r9:    "r9",
	r10:   "r10",
	r11:   "r11",
	r12:   "r12",
	r13:   "r13",
	r14:   "r14",
	r15:   "r15",
	xmm0:  "xmm0",
	xmm1:  "xmm1",
	xmm2:  "xmm2",
	xmm3:  "xmm3",
	xmm4:  "xmm4",
	xmm5:  "xmm5",
	xmm6:  "xmm6",
	xmm7:  "xmm7",
	xmm8:  "xmm8",
	xmm9:  "xmm9",
	xmm10: "xmm10",
	xmm11: "xmm11",
	xmm12: "xmm12",
	xmm13: "xmm13",
	xmm14: "xmm14",
	xmm15: "xmm15",
}

func formatVRegSized(r regalloc.VReg, _64 bool) string {
	if r.IsRealReg() {
		if r.RegType() == regalloc.RegTypeInt {
			rr := r.RealReg()
			orig := regNames[rr]
			if rr <= rdi {
				if _64 {
					return "%" + orig
				} else {
					return "%e" + orig[1:]
				}
			} else {
				if _64 {
					return "%" + orig
				} else {
					return "%" + orig + "d"
				}
			}
		} else {
			return "%" + regNames[r.RealReg()]
		}
	} else {
		if r.RegType() == regalloc.RegTypeInt {
			if _64 {
				return fmt.Sprintf("%%r%d?", r.ID())
			} else {
				return fmt.Sprintf("%%r%dd?", r.ID())
			}
		} else {
			return fmt.Sprintf("%%xmm%d?", r.ID())
		}
	}
}
