package amd64

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

func (i *instruction) encode(c backend.Compiler) (needsLabelResolution bool) {
	switch kind := i.kind; kind {
	case nop0, sourceOffsetInfo, defineUninitializedReg, fcvtToSintSequence, fcvtToUintSequence, nopUseReg:
	case ret:
		encodeRet(c)
	case imm:
		dst := regEncodings[i.op2.reg().RealReg()]
		con := i.u1
		if i.b1 { // 64 bit.
			if lower32willSignExtendTo64(con) {
				// Sign extend mov(imm32).
				encodeRegReg(c,
					legacyPrefixesNone,
					0xc7, 1,
					0,
					dst,
					rexInfo(0).setW(),
				)
				c.Emit4Bytes(uint32(con))
			} else {
				c.EmitByte(rexEncodingW | dst.rexBit())
				c.EmitByte(0xb8 | dst.encoding())
				c.Emit8Bytes(con)
			}
		} else {
			if dst.rexBit() > 0 {
				c.EmitByte(rexEncodingDefault | 0x1)
			}
			c.EmitByte(0xb8 | dst.encoding())
			c.Emit4Bytes(uint32(con))
		}

	case aluRmiR:
		var rex rexInfo
		if i.b1 {
			rex = rex.setW()
		} else {
			rex = rex.clearW()
		}

		dst := regEncodings[i.op2.reg().RealReg()]

		aluOp := aluRmiROpcode(i.u1)
		if aluOp == aluRmiROpcodeMul {
			op1 := i.op1
			const regMemOpc, regMemOpcNum = 0x0FAF, 2
			switch op1.kind {
			case operandKindReg:
				src := regEncodings[op1.reg().RealReg()]
				encodeRegReg(c, legacyPrefixesNone, regMemOpc, regMemOpcNum, dst, src, rex)
			case operandKindMem:
				m := i.op1.addressMode()
				encodeRegMem(c, legacyPrefixesNone, regMemOpc, regMemOpcNum, dst, m, rex)
			case operandKindImm32:
				imm8 := lower8willSignExtendTo32(op1.imm32())
				var opc uint32
				if imm8 {
					opc = 0x6b
				} else {
					opc = 0x69
				}
				encodeRegReg(c, legacyPrefixesNone, opc, 1, dst, dst, rex)
				if imm8 {
					c.EmitByte(byte(op1.imm32()))
				} else {
					c.Emit4Bytes(op1.imm32())
				}
			default:
				panic("BUG: invalid operand kind")
			}
		} else {
			const opcodeNum = 1
			var opcR, opcM, subOpcImm uint32
			switch aluOp {
			case aluRmiROpcodeAdd:
				opcR, opcM, subOpcImm = 0x01, 0x03, 0x0
			case aluRmiROpcodeSub:
				opcR, opcM, subOpcImm = 0x29, 0x2b, 0x5
			case aluRmiROpcodeAnd:
				opcR, opcM, subOpcImm = 0x21, 0x23, 0x4
			case aluRmiROpcodeOr:
				opcR, opcM, subOpcImm = 0x09, 0x0b, 0x1
			case aluRmiROpcodeXor:
				opcR, opcM, subOpcImm = 0x31, 0x33, 0x6
			default:
				panic("BUG: invalid aluRmiROpcode")
			}

			op1 := i.op1
			switch op1.kind {
			case operandKindReg:
				src := regEncodings[op1.reg().RealReg()]
				encodeRegReg(c, legacyPrefixesNone, opcR, opcodeNum, src, dst, rex)
			case operandKindMem:
				m := i.op1.addressMode()
				encodeRegMem(c, legacyPrefixesNone, opcM, opcodeNum, dst, m, rex)
			case operandKindImm32:
				imm8 := lower8willSignExtendTo32(op1.imm32())
				var opc uint32
				if imm8 {
					opc = 0x83
				} else {
					opc = 0x81
				}
				encodeRegReg(c, legacyPrefixesNone, opc, opcodeNum, regEnc(subOpcImm), dst, rex)
				if imm8 {
					c.EmitByte(byte(op1.imm32()))
				} else {
					c.Emit4Bytes(op1.imm32())
				}
			default:
				panic("BUG: invalid operand kind")
			}
		}

	case movRR:
		src := regEncodings[i.op1.reg().RealReg()]
		dst := regEncodings[i.op2.reg().RealReg()]
		var rex rexInfo
		if i.b1 {
			rex = rex.setW()
		} else {
			rex = rex.clearW()
		}
		encodeRegReg(c, legacyPrefixesNone, 0x89, 1, src, dst, rex)

	case xmmRmR, blendvpd:
		op := sseOpcode(i.u1)
		var legPrex legacyPrefixes
		var opcode uint32
		var opcodeNum uint32
		switch op {
		case sseOpcodeAddps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0F58, 2
		case sseOpcodeAddpd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F58, 2
		case sseOpcodeAddss:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF3, 0x0F58, 2
		case sseOpcodeAddsd:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF2, 0x0F58, 2
		case sseOpcodeAndps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0F54, 2
		case sseOpcodeAndpd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F54, 2
		case sseOpcodeAndnps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0F55, 2
		case sseOpcodeAndnpd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F55, 2
		case sseOpcodeBlendvps:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3814, 3
		case sseOpcodeBlendvpd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3815, 3
		case sseOpcodeDivps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0F5E, 2
		case sseOpcodeDivpd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F5E, 2
		case sseOpcodeDivss:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF3, 0x0F5E, 2
		case sseOpcodeDivsd:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF2, 0x0F5E, 2
		case sseOpcodeMaxps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0F5F, 2
		case sseOpcodeMaxpd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F5F, 2
		case sseOpcodeMaxss:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF3, 0x0F5F, 2
		case sseOpcodeMaxsd:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF2, 0x0F5F, 2
		case sseOpcodeMinps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0F5D, 2
		case sseOpcodeMinpd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F5D, 2
		case sseOpcodeMinss:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF3, 0x0F5D, 2
		case sseOpcodeMinsd:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF2, 0x0F5D, 2
		case sseOpcodeMovlhps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0F16, 2
		case sseOpcodeMovsd:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF2, 0x0F10, 2
		case sseOpcodeMulps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0F59, 2
		case sseOpcodeMulpd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F59, 2
		case sseOpcodeMulss:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF3, 0x0F59, 2
		case sseOpcodeMulsd:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF2, 0x0F59, 2
		case sseOpcodeOrpd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F56, 2
		case sseOpcodeOrps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0F56, 2
		case sseOpcodePackssdw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F6B, 2
		case sseOpcodePacksswb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F63, 2
		case sseOpcodePackusdw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F382B, 3
		case sseOpcodePackuswb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F67, 2
		case sseOpcodePaddb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FFC, 2
		case sseOpcodePaddd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FFE, 2
		case sseOpcodePaddq:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FD4, 2
		case sseOpcodePaddw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FFD, 2
		case sseOpcodePaddsb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FEC, 2
		case sseOpcodePaddsw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FED, 2
		case sseOpcodePaddusb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FDC, 2
		case sseOpcodePaddusw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FDD, 2
		case sseOpcodePand:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FDB, 2
		case sseOpcodePandn:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FDF, 2
		case sseOpcodePavgb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FE0, 2
		case sseOpcodePavgw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FE3, 2
		case sseOpcodePcmpeqb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F74, 2
		case sseOpcodePcmpeqw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F75, 2
		case sseOpcodePcmpeqd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F76, 2
		case sseOpcodePcmpeqq:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3829, 3
		case sseOpcodePcmpgtb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F64, 2
		case sseOpcodePcmpgtw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F65, 2
		case sseOpcodePcmpgtd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F66, 2
		case sseOpcodePcmpgtq:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3837, 3
		case sseOpcodePmaddwd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FF5, 2
		case sseOpcodePmaxsb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F383C, 3
		case sseOpcodePmaxsw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FEE, 2
		case sseOpcodePmaxsd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F383D, 3
		case sseOpcodePmaxub:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FDE, 2
		case sseOpcodePmaxuw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F383E, 3
		case sseOpcodePmaxud:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F383F, 3
		case sseOpcodePminsb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3838, 3
		case sseOpcodePminsw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FEA, 2
		case sseOpcodePminsd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3839, 3
		case sseOpcodePminub:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FDA, 2
		case sseOpcodePminuw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F383A, 3
		case sseOpcodePminud:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F383B, 3
		case sseOpcodePmulld:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3840, 3
		case sseOpcodePmullw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FD5, 2
		case sseOpcodePmuludq:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FF4, 2
		case sseOpcodePor:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FEB, 2
		case sseOpcodePshufb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3800, 3
		case sseOpcodePsubb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FF8, 2
		case sseOpcodePsubd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FFA, 2
		case sseOpcodePsubq:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FFB, 2
		case sseOpcodePsubw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FF9, 2
		case sseOpcodePsubsb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FE8, 2
		case sseOpcodePsubsw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FE9, 2
		case sseOpcodePsubusb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FD8, 2
		case sseOpcodePsubusw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FD9, 2
		case sseOpcodePunpckhbw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F68, 2
		case sseOpcodePunpcklbw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F60, 2
		case sseOpcodePxor:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FEF, 2
		case sseOpcodeSubps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0F5C, 2
		case sseOpcodeSubpd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F5C, 2
		case sseOpcodeSubss:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF3, 0x0F5C, 2
		case sseOpcodeSubsd:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF2, 0x0F5C, 2
		case sseOpcodeXorps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0F57, 2
		case sseOpcodeXorpd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F57, 2
		case sseOpcodePmulhrsw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F380B, 3
		case sseOpcodeUnpcklps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0F14, 2
		case sseOpcodePmaddubsw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3804, 3
		default:
			if kind == blendvpd {
				legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3815, 3
			} else {
				panic(fmt.Sprintf("Unsupported sseOpcode: %s", op))
			}
		}

		dst := regEncodings[i.op2.reg().RealReg()]

		rex := rexInfo(0).clearW()
		op1 := i.op1
		if op1.kind == operandKindReg {
			src := regEncodings[op1.reg().RealReg()]
			encodeRegReg(c, legPrex, opcode, opcodeNum, dst, src, rex)
		} else if i.op1.kind == operandKindMem {
			m := i.op1.addressMode()
			encodeRegMem(c, legPrex, opcode, opcodeNum, dst, m, rex)
		} else {
			panic("BUG: invalid operand kind")
		}

	case gprToXmm:
		var legPrefix legacyPrefixes
		var opcode uint32
		const opcodeNum = 2
		switch sseOpcode(i.u1) {
		case sseOpcodeMovd, sseOpcodeMovq:
			legPrefix, opcode = legacyPrefixes0x66, 0x0f6e
		case sseOpcodeCvtsi2ss:
			legPrefix, opcode = legacyPrefixes0xF3, 0x0f2a
		case sseOpcodeCvtsi2sd:
			legPrefix, opcode = legacyPrefixes0xF2, 0x0f2a
		default:
			panic(fmt.Sprintf("Unsupported sseOpcode: %s", sseOpcode(i.u1)))
		}

		var rex rexInfo
		if i.b1 {
			rex = rex.setW()
		} else {
			rex = rex.clearW()
		}
		dst := regEncodings[i.op2.reg().RealReg()]

		op1 := i.op1
		if op1.kind == operandKindReg {
			src := regEncodings[op1.reg().RealReg()]
			encodeRegReg(c, legPrefix, opcode, opcodeNum, dst, src, rex)
		} else if i.op1.kind == operandKindMem {
			m := i.op1.addressMode()
			encodeRegMem(c, legPrefix, opcode, opcodeNum, dst, m, rex)
		} else {
			panic("BUG: invalid operand kind")
		}

	case xmmUnaryRmR:
		var prefix legacyPrefixes
		var opcode uint32
		var opcodeNum uint32
		op := sseOpcode(i.u1)
		switch op {
		case sseOpcodeCvtss2sd:
			prefix, opcode, opcodeNum = legacyPrefixes0xF3, 0x0F5A, 2
		case sseOpcodeCvtsd2ss:
			prefix, opcode, opcodeNum = legacyPrefixes0xF2, 0x0F5A, 2
		case sseOpcodeMovaps:
			prefix, opcode, opcodeNum = legacyPrefixesNone, 0x0F28, 2
		case sseOpcodeMovapd:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F28, 2
		case sseOpcodeMovdqa:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F6F, 2
		case sseOpcodeMovdqu:
			prefix, opcode, opcodeNum = legacyPrefixes0xF3, 0x0F6F, 2
		case sseOpcodeMovsd:
			prefix, opcode, opcodeNum = legacyPrefixes0xF2, 0x0F10, 2
		case sseOpcodeMovss:
			prefix, opcode, opcodeNum = legacyPrefixes0xF3, 0x0F10, 2
		case sseOpcodeMovups:
			prefix, opcode, opcodeNum = legacyPrefixesNone, 0x0F10, 2
		case sseOpcodeMovupd:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F10, 2
		case sseOpcodePabsb:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F381C, 3
		case sseOpcodePabsw:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F381D, 3
		case sseOpcodePabsd:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F381E, 3
		case sseOpcodePmovsxbd:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3821, 3
		case sseOpcodePmovsxbw:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3820, 3
		case sseOpcodePmovsxbq:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3822, 3
		case sseOpcodePmovsxwd:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3823, 3
		case sseOpcodePmovsxwq:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3824, 3
		case sseOpcodePmovsxdq:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3825, 3
		case sseOpcodePmovzxbd:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3831, 3
		case sseOpcodePmovzxbw:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3830, 3
		case sseOpcodePmovzxbq:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3832, 3
		case sseOpcodePmovzxwd:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3833, 3
		case sseOpcodePmovzxwq:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3834, 3
		case sseOpcodePmovzxdq:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3835, 3
		case sseOpcodeSqrtps:
			prefix, opcode, opcodeNum = legacyPrefixesNone, 0x0F51, 2
		case sseOpcodeSqrtpd:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F51, 2
		case sseOpcodeSqrtss:
			prefix, opcode, opcodeNum = legacyPrefixes0xF3, 0x0F51, 2
		case sseOpcodeSqrtsd:
			prefix, opcode, opcodeNum = legacyPrefixes0xF2, 0x0F51, 2
		case sseOpcodeXorps:
			prefix, opcode, opcodeNum = legacyPrefixesNone, 0x0F57, 2
		case sseOpcodeXorpd:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F57, 2
		case sseOpcodeCvtdq2ps:
			prefix, opcode, opcodeNum = legacyPrefixesNone, 0x0F5B, 2
		case sseOpcodeCvtdq2pd:
			prefix, opcode, opcodeNum = legacyPrefixes0xF3, 0x0FE6, 2
		case sseOpcodeCvtps2pd:
			prefix, opcode, opcodeNum = legacyPrefixesNone, 0x0F5A, 2
		case sseOpcodeCvtpd2ps:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0F5A, 2
		case sseOpcodeCvttps2dq:
			prefix, opcode, opcodeNum = legacyPrefixes0xF3, 0x0F5B, 2
		case sseOpcodeCvttpd2dq:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0FE6, 2
		default:
			panic(fmt.Sprintf("Unsupported sseOpcode: %s", op))
		}

		dst := regEncodings[i.op2.reg().RealReg()]

		rex := rexInfo(0).clearW()
		op1 := i.op1
		if op1.kind == operandKindReg {
			src := regEncodings[op1.reg().RealReg()]
			encodeRegReg(c, prefix, opcode, opcodeNum, dst, src, rex)
		} else if i.op1.kind == operandKindMem {
			m := i.op1.addressMode()
			needsLabelResolution = encodeRegMem(c, prefix, opcode, opcodeNum, dst, m, rex)
		} else {
			panic("BUG: invalid operand kind")
		}

	case xmmUnaryRmRImm:
		var prefix legacyPrefixes
		var opcode uint32
		var opcodeNum uint32
		op := sseOpcode(i.u1)
		switch op {
		case sseOpcodeRoundps:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0f3a08, 3
		case sseOpcodeRoundss:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0f3a0a, 3
		case sseOpcodeRoundpd:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0f3a09, 3
		case sseOpcodeRoundsd:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0f3a0b, 3
		}
		rex := rexInfo(0).clearW()
		dst := regEncodings[i.op2.reg().RealReg()]
		op1 := i.op1
		if op1.kind == operandKindReg {
			src := regEncodings[op1.reg().RealReg()]
			encodeRegReg(c, prefix, opcode, opcodeNum, dst, src, rex)
		} else if i.op1.kind == operandKindMem {
			m := i.op1.addressMode()
			encodeRegMem(c, prefix, opcode, opcodeNum, dst, m, rex)
		} else {
			panic("BUG: invalid operand kind")
		}

		c.EmitByte(byte(i.u2))

	case unaryRmR:
		var prefix legacyPrefixes
		var opcode uint32
		var opcodeNum uint32
		op := unaryRmROpcode(i.u1)
		// We assume size is either 32 or 64.
		switch op {
		case unaryRmROpcodeBsr:
			prefix, opcode, opcodeNum = legacyPrefixesNone, 0x0fbd, 2
		case unaryRmROpcodeBsf:
			prefix, opcode, opcodeNum = legacyPrefixesNone, 0x0fbc, 2
		case unaryRmROpcodeLzcnt:
			prefix, opcode, opcodeNum = legacyPrefixes0xF3, 0x0fbd, 2
		case unaryRmROpcodeTzcnt:
			prefix, opcode, opcodeNum = legacyPrefixes0xF3, 0x0fbc, 2
		case unaryRmROpcodePopcnt:
			prefix, opcode, opcodeNum = legacyPrefixes0xF3, 0x0fb8, 2
		default:
			panic(fmt.Sprintf("Unsupported unaryRmROpcode: %s", op))
		}

		dst := regEncodings[i.op2.reg().RealReg()]

		rex := rexInfo(0)
		if i.b1 { // 64 bit.
			rex = rexInfo(0).setW()
		} else {
			rex = rexInfo(0).clearW()
		}
		op1 := i.op1
		if op1.kind == operandKindReg {
			src := regEncodings[op1.reg().RealReg()]
			encodeRegReg(c, prefix, opcode, opcodeNum, dst, src, rex)
		} else if i.op1.kind == operandKindMem {
			m := i.op1.addressMode()
			encodeRegMem(c, prefix, opcode, opcodeNum, dst, m, rex)
		} else {
			panic("BUG: invalid operand kind")
		}

	case not:
		var prefix legacyPrefixes
		src := regEncodings[i.op1.reg().RealReg()]
		rex := rexInfo(0)
		if i.b1 { // 64 bit.
			rex = rexInfo(0).setW()
		} else {
			rex = rexInfo(0).clearW()
		}
		subopcode := uint8(2)
		encodeEncEnc(c, prefix, 0xf7, 1, subopcode, uint8(src), rex)

	case neg:
		var prefix legacyPrefixes
		src := regEncodings[i.op1.reg().RealReg()]
		rex := rexInfo(0)
		if i.b1 { // 64 bit.
			rex = rexInfo(0).setW()
		} else {
			rex = rexInfo(0).clearW()
		}
		subopcode := uint8(3)
		encodeEncEnc(c, prefix, 0xf7, 1, subopcode, uint8(src), rex)

	case div:
		rex := rexInfo(0)
		if i.b1 { // 64 bit.
			rex = rexInfo(0).setW()
		} else {
			rex = rexInfo(0).clearW()
		}
		var subopcode uint8
		if i.u1 != 0 { // Signed.
			subopcode = 7
		} else {
			subopcode = 6
		}

		divisor := i.op1
		if divisor.kind == operandKindReg {
			src := regEncodings[divisor.reg().RealReg()]
			encodeEncEnc(c, legacyPrefixesNone, 0xf7, 1, subopcode, uint8(src), rex)
		} else if divisor.kind == operandKindMem {
			m := divisor.addressMode()
			encodeEncMem(c, legacyPrefixesNone, 0xf7, 1, subopcode, m, rex)
		} else {
			panic("BUG: invalid operand kind")
		}

	case mulHi:
		var prefix legacyPrefixes
		rex := rexInfo(0)
		if i.b1 { // 64 bit.
			rex = rexInfo(0).setW()
		} else {
			rex = rexInfo(0).clearW()
		}

		signed := i.u1 != 0
		var subopcode uint8
		if signed {
			subopcode = 5
		} else {
			subopcode = 4
		}

		// src1 is implicitly rax,
		// dst_lo is implicitly rax,
		// dst_hi is implicitly rdx.
		src2 := i.op1
		if src2.kind == operandKindReg {
			src := regEncodings[src2.reg().RealReg()]
			encodeEncEnc(c, prefix, 0xf7, 1, subopcode, uint8(src), rex)
		} else if src2.kind == operandKindMem {
			m := src2.addressMode()
			encodeEncMem(c, prefix, 0xf7, 1, subopcode, m, rex)
		} else {
			panic("BUG: invalid operand kind")
		}

	case signExtendData:
		if i.b1 { // 64 bit.
			c.EmitByte(0x48)
			c.EmitByte(0x99)
		} else {
			c.EmitByte(0x99)
		}
	case movzxRmR, movsxRmR:
		signed := i.kind == movsxRmR

		ext := extMode(i.u1)
		var opcode uint32
		var opcodeNum uint32
		var rex rexInfo
		switch ext {
		case extModeBL:
			if signed {
				opcode, opcodeNum, rex = 0x0fbe, 2, rex.clearW()
			} else {
				opcode, opcodeNum, rex = 0x0fb6, 2, rex.clearW()
			}
		case extModeBQ:
			if signed {
				opcode, opcodeNum, rex = 0x0fbe, 2, rex.setW()
			} else {
				opcode, opcodeNum, rex = 0x0fb6, 2, rex.setW()
			}
		case extModeWL:
			if signed {
				opcode, opcodeNum, rex = 0x0fbf, 2, rex.clearW()
			} else {
				opcode, opcodeNum, rex = 0x0fb7, 2, rex.clearW()
			}
		case extModeWQ:
			if signed {
				opcode, opcodeNum, rex = 0x0fbf, 2, rex.setW()
			} else {
				opcode, opcodeNum, rex = 0x0fb7, 2, rex.setW()
			}
		case extModeLQ:
			if signed {
				opcode, opcodeNum, rex = 0x63, 1, rex.setW()
			} else {
				opcode, opcodeNum, rex = 0x8b, 1, rex.clearW()
			}
		default:
			panic("BUG: invalid extMode")
		}

		op := i.op1
		dst := regEncodings[i.op2.reg().RealReg()]
		switch op.kind {
		case operandKindReg:
			src := regEncodings[op.reg().RealReg()]
			if ext == extModeBL || ext == extModeBQ {
				// Some destinations must be encoded with REX.R = 1.
				if e := src.encoding(); e >= 4 && e <= 7 {
					rex = rex.always()
				}
			}
			encodeRegReg(c, legacyPrefixesNone, opcode, opcodeNum, dst, src, rex)
		case operandKindMem:
			m := op.addressMode()
			encodeRegMem(c, legacyPrefixesNone, opcode, opcodeNum, dst, m, rex)
		default:
			panic("BUG: invalid operand kind")
		}

	case mov64MR:
		m := i.op1.addressMode()
		encodeLoad64(c, m, i.op2.reg().RealReg())

	case lea:
		needsLabelResolution = true
		dst := regEncodings[i.op2.reg().RealReg()]
		rex := rexInfo(0).setW()
		const opcode, opcodeNum = 0x8d, 1
		switch i.op1.kind {
		case operandKindMem:
			a := i.op1.addressMode()
			encodeRegMem(c, legacyPrefixesNone, opcode, opcodeNum, dst, a, rex)
		case operandKindLabel:
			rex.encode(c, regRexBit(byte(dst)), 0)
			c.EmitByte(byte((opcode) & 0xff))

			// Indicate "LEAQ [RIP + 32bit displacement].
			// https://wiki.osdev.org/X86-64_Instruction_Encoding#32.2F64-bit_addressing
			c.EmitByte(encodeModRM(0b00, dst.encoding(), 0b101))

			// This will be resolved later, so we just emit a placeholder (0xffffffff for testing).
			c.Emit4Bytes(0xffffffff)
		default:
			panic("BUG: invalid operand kind")
		}

	case movRM:
		m := i.op2.addressMode()
		src := regEncodings[i.op1.reg().RealReg()]

		var rex rexInfo
		switch i.u1 {
		case 1:
			if e := src.encoding(); e >= 4 && e <= 7 {
				rex = rex.always()
			}
			encodeRegMem(c, legacyPrefixesNone, 0x88, 1, src, m, rex.clearW())
		case 2:
			encodeRegMem(c, legacyPrefixes0x66, 0x89, 1, src, m, rex.clearW())
		case 4:
			encodeRegMem(c, legacyPrefixesNone, 0x89, 1, src, m, rex.clearW())
		case 8:
			encodeRegMem(c, legacyPrefixesNone, 0x89, 1, src, m, rex.setW())
		default:
			panic(fmt.Sprintf("BUG: invalid size %d: %s", i.u1, i.String()))
		}

	case shiftR:
		src := regEncodings[i.op2.reg().RealReg()]
		amount := i.op1

		var opcode uint32
		var prefix legacyPrefixes
		rex := rexInfo(0)
		if i.b1 { // 64 bit.
			rex = rexInfo(0).setW()
		} else {
			rex = rexInfo(0).clearW()
		}

		switch amount.kind {
		case operandKindReg:
			if amount.reg() != rcxVReg {
				panic("BUG: invalid reg operand: must be rcx")
			}
			opcode, prefix = 0xd3, legacyPrefixesNone
			encodeEncEnc(c, prefix, opcode, 1, uint8(i.u1), uint8(src), rex)
		case operandKindImm32:
			opcode, prefix = 0xc1, legacyPrefixesNone
			encodeEncEnc(c, prefix, opcode, 1, uint8(i.u1), uint8(src), rex)
			c.EmitByte(byte(amount.imm32()))
		default:
			panic("BUG: invalid operand kind")
		}
	case xmmRmiReg:
		const legPrefix = legacyPrefixes0x66
		rex := rexInfo(0).clearW()
		dst := regEncodings[i.op2.reg().RealReg()]

		var opcode uint32
		var regDigit uint8

		op := sseOpcode(i.u1)
		op1 := i.op1
		if i.op1.kind == operandKindImm32 {
			switch op {
			case sseOpcodePsllw:
				opcode, regDigit = 0x0f71, 6
			case sseOpcodePslld:
				opcode, regDigit = 0x0f72, 6
			case sseOpcodePsllq:
				opcode, regDigit = 0x0f73, 6
			case sseOpcodePsraw:
				opcode, regDigit = 0x0f71, 4
			case sseOpcodePsrad:
				opcode, regDigit = 0x0f72, 4
			case sseOpcodePsrlw:
				opcode, regDigit = 0x0f71, 2
			case sseOpcodePsrld:
				opcode, regDigit = 0x0f72, 2
			case sseOpcodePsrlq:
				opcode, regDigit = 0x0f73, 2
			default:
				panic("invalid opcode")
			}

			encodeEncEnc(c, legPrefix, opcode, 2, regDigit, uint8(dst), rex)
			imm32 := op1.imm32()
			if imm32 > 0xff&imm32 {
				panic("immediate value does not fit 1 byte")
			}
			c.EmitByte(uint8(imm32))
		} else {
			switch op {
			case sseOpcodePsllw:
				opcode = 0x0ff1
			case sseOpcodePslld:
				opcode = 0x0ff2
			case sseOpcodePsllq:
				opcode = 0x0ff3
			case sseOpcodePsraw:
				opcode = 0x0fe1
			case sseOpcodePsrad:
				opcode = 0x0fe2
			case sseOpcodePsrlw:
				opcode = 0x0fd1
			case sseOpcodePsrld:
				opcode = 0x0fd2
			case sseOpcodePsrlq:
				opcode = 0x0fd3
			default:
				panic("invalid opcode")
			}

			if op1.kind == operandKindReg {
				reg := regEncodings[op1.reg().RealReg()]
				encodeRegReg(c, legPrefix, opcode, 2, dst, reg, rex)
			} else if op1.kind == operandKindMem {
				m := op1.addressMode()
				encodeRegMem(c, legPrefix, opcode, 2, dst, m, rex)
			} else {
				panic("BUG: invalid operand kind")
			}
		}

	case cmpRmiR:
		var opcode uint32
		isCmp := i.u1 != 0
		rex := rexInfo(0)
		_64 := i.b1
		if _64 { // 64 bit.
			rex = rex.setW()
		} else {
			rex = rex.clearW()
		}
		dst := regEncodings[i.op2.reg().RealReg()]
		op1 := i.op1
		switch op1.kind {
		case operandKindReg:
			reg := regEncodings[op1.reg().RealReg()]
			if isCmp {
				opcode = 0x39
			} else {
				opcode = 0x85
			}
			// Here we swap the encoding of the operands for CMP to be consistent with the output of LLVM/GCC.
			encodeRegReg(c, legacyPrefixesNone, opcode, 1, reg, dst, rex)

		case operandKindMem:
			if isCmp {
				opcode = 0x3b
			} else {
				opcode = 0x85
			}
			m := op1.addressMode()
			encodeRegMem(c, legacyPrefixesNone, opcode, 1, dst, m, rex)

		case operandKindImm32:
			imm32 := op1.imm32()
			useImm8 := isCmp && lower8willSignExtendTo32(imm32)
			var subopcode uint8

			switch {
			case isCmp && useImm8:
				opcode, subopcode = 0x83, 7
			case isCmp && !useImm8:
				opcode, subopcode = 0x81, 7
			default:
				opcode, subopcode = 0xf7, 0
			}
			encodeEncEnc(c, legacyPrefixesNone, opcode, 1, subopcode, uint8(dst), rex)
			if useImm8 {
				c.EmitByte(uint8(imm32))
			} else {
				c.Emit4Bytes(imm32)
			}

		default:
			panic("BUG: invalid operand kind")
		}
	case setcc:
		cc := cond(i.u1)
		dst := regEncodings[i.op2.reg().RealReg()]
		rex := rexInfo(0).clearW().always()
		opcode := uint32(0x0f90) + uint32(cc)
		encodeEncEnc(c, legacyPrefixesNone, opcode, 2, 0, uint8(dst), rex)
	case cmove:
		cc := cond(i.u1)
		dst := regEncodings[i.op2.reg().RealReg()]
		rex := rexInfo(0)
		if i.b1 { // 64 bit.
			rex = rex.setW()
		} else {
			rex = rex.clearW()
		}
		opcode := uint32(0x0f40) + uint32(cc)
		src := i.op1
		switch src.kind {
		case operandKindReg:
			srcReg := regEncodings[src.reg().RealReg()]
			encodeRegReg(c, legacyPrefixesNone, opcode, 2, dst, srcReg, rex)
		case operandKindMem:
			m := src.addressMode()
			encodeRegMem(c, legacyPrefixesNone, opcode, 2, dst, m, rex)
		default:
			panic("BUG: invalid operand kind")
		}
	case push64:
		op := i.op1

		switch op.kind {
		case operandKindReg:
			dst := regEncodings[op.reg().RealReg()]
			if dst.rexBit() > 0 {
				c.EmitByte(rexEncodingDefault | 0x1)
			}
			c.EmitByte(0x50 | dst.encoding())
		case operandKindMem:
			m := op.addressMode()
			encodeRegMem(
				c, legacyPrefixesNone, 0xff, 1, regEnc(6), m, rexInfo(0).clearW(),
			)
		case operandKindImm32:
			c.EmitByte(0x68)
			c.Emit4Bytes(op.imm32())
		default:
			panic("BUG: invalid operand kind")
		}

	case pop64:
		dst := regEncodings[i.op1.reg().RealReg()]
		if dst.rexBit() > 0 {
			c.EmitByte(rexEncodingDefault | 0x1)
		}
		c.EmitByte(0x58 | dst.encoding())

	case xmmMovRM:
		var legPrefix legacyPrefixes
		var opcode uint32
		const opcodeNum = 2
		switch sseOpcode(i.u1) {
		case sseOpcodeMovaps:
			legPrefix, opcode = legacyPrefixesNone, 0x0f29
		case sseOpcodeMovapd:
			legPrefix, opcode = legacyPrefixes0x66, 0x0f29
		case sseOpcodeMovdqa:
			legPrefix, opcode = legacyPrefixes0x66, 0x0f7f
		case sseOpcodeMovdqu:
			legPrefix, opcode = legacyPrefixes0xF3, 0x0f7f
		case sseOpcodeMovss:
			legPrefix, opcode = legacyPrefixes0xF3, 0x0f11
		case sseOpcodeMovsd:
			legPrefix, opcode = legacyPrefixes0xF2, 0x0f11
		case sseOpcodeMovups:
			legPrefix, opcode = legacyPrefixesNone, 0x0f11
		case sseOpcodeMovupd:
			legPrefix, opcode = legacyPrefixes0x66, 0x0f11
		default:
			panic(fmt.Sprintf("Unsupported sseOpcode: %s", sseOpcode(i.u1)))
		}

		dst := regEncodings[i.op1.reg().RealReg()]
		encodeRegMem(c, legPrefix, opcode, opcodeNum, dst, i.op2.addressMode(), rexInfo(0).clearW())
	case xmmLoadConst:
		panic("TODO")
	case xmmToGpr:
		var legPrefix legacyPrefixes
		var opcode uint32
		var argSwap bool
		const opcodeNum = 2
		switch sseOpcode(i.u1) {
		case sseOpcodeMovd, sseOpcodeMovq:
			legPrefix, opcode, argSwap = legacyPrefixes0x66, 0x0f7e, false
		case sseOpcodeMovmskps:
			legPrefix, opcode, argSwap = legacyPrefixesNone, 0x0f50, true
		case sseOpcodeMovmskpd:
			legPrefix, opcode, argSwap = legacyPrefixes0x66, 0x0f50, true
		case sseOpcodePmovmskb:
			legPrefix, opcode, argSwap = legacyPrefixes0x66, 0x0fd7, true
		case sseOpcodeCvttss2si:
			legPrefix, opcode, argSwap = legacyPrefixes0xF3, 0x0f2c, true
		case sseOpcodeCvttsd2si:
			legPrefix, opcode, argSwap = legacyPrefixes0xF2, 0x0f2c, true
		default:
			panic(fmt.Sprintf("Unsupported sseOpcode: %s", sseOpcode(i.u1)))
		}

		var rex rexInfo
		if i.b1 {
			rex = rex.setW()
		} else {
			rex = rex.clearW()
		}
		src := regEncodings[i.op1.reg().RealReg()]
		dst := regEncodings[i.op2.reg().RealReg()]
		if argSwap {
			src, dst = dst, src
		}
		encodeRegReg(c, legPrefix, opcode, opcodeNum, src, dst, rex)

	case cvtUint64ToFloatSeq:
		panic("TODO")
	case cvtFloatToSintSeq:
		panic("TODO")
	case cvtFloatToUintSeq:
		panic("TODO")
	case xmmMinMaxSeq:
		panic("TODO")
	case xmmCmpRmR:
		var prefix legacyPrefixes
		var opcode uint32
		var opcodeNum uint32
		rex := rexInfo(0)
		_64 := i.b1
		if _64 { // 64 bit.
			rex = rex.setW()
		} else {
			rex = rex.clearW()
		}

		op := sseOpcode(i.u1)
		switch op {
		case sseOpcodePtest:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0f3817, 3
		case sseOpcodeUcomisd:
			prefix, opcode, opcodeNum = legacyPrefixes0x66, 0x0f2e, 2
		case sseOpcodeUcomiss:
			prefix, opcode, opcodeNum = legacyPrefixesNone, 0x0f2e, 2
		default:
			panic(fmt.Sprintf("Unsupported sseOpcode: %s", op))
		}

		dst := regEncodings[i.op2.reg().RealReg()]
		op1 := i.op1
		switch op1.kind {
		case operandKindReg:
			reg := regEncodings[op1.reg().RealReg()]
			encodeRegReg(c, prefix, opcode, opcodeNum, dst, reg, rex)

		case operandKindMem:
			m := op1.addressMode()
			encodeRegMem(c, prefix, opcode, opcodeNum, dst, m, rex)

		default:
			panic("BUG: invalid operand kind")
		}
	case xmmRmRImm:
		op := sseOpcode(i.u1)
		var legPrex legacyPrefixes
		var opcode uint32
		var opcodeNum uint32
		var swap bool
		switch op {
		case sseOpcodeCmpps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0FC2, 2
		case sseOpcodeCmppd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FC2, 2
		case sseOpcodeCmpss:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF3, 0x0FC2, 2
		case sseOpcodeCmpsd:
			legPrex, opcode, opcodeNum = legacyPrefixes0xF2, 0x0FC2, 2
		case sseOpcodeInsertps:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3A21, 3
		case sseOpcodePalignr:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3A0F, 3
		case sseOpcodePinsrb:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3A20, 3
		case sseOpcodePinsrw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FC4, 2
		case sseOpcodePinsrd, sseOpcodePinsrq:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3A22, 3
		case sseOpcodePextrb:
			swap = true
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3A14, 3
		case sseOpcodePextrw:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0FC5, 2
		case sseOpcodePextrd, sseOpcodePextrq:
			swap = true
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3A16, 3
		case sseOpcodePshufd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F70, 2
		case sseOpcodeRoundps:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3A08, 3
		case sseOpcodeRoundpd:
			legPrex, opcode, opcodeNum = legacyPrefixes0x66, 0x0F3A09, 3
		case sseOpcodeShufps:
			legPrex, opcode, opcodeNum = legacyPrefixesNone, 0x0FC6, 2
		default:
			panic(fmt.Sprintf("Unsupported sseOpcode: %s", op))
		}

		dst := regEncodings[i.op2.reg().RealReg()]

		var rex rexInfo
		if op == sseOpcodePextrq || op == sseOpcodePinsrq {
			rex = rexInfo(0).setW()
		} else {
			rex = rexInfo(0).clearW()
		}
		op1 := i.op1
		if op1.kind == operandKindReg {
			src := regEncodings[op1.reg().RealReg()]
			if swap {
				src, dst = dst, src
			}
			encodeRegReg(c, legPrex, opcode, opcodeNum, dst, src, rex)
		} else if i.op1.kind == operandKindMem {
			if swap {
				panic("BUG: this is not possible to encode")
			}
			m := i.op1.addressMode()
			encodeRegMem(c, legPrex, opcode, opcodeNum, dst, m, rex)
		} else {
			panic("BUG: invalid operand kind")
		}

		c.EmitByte(byte(i.u2))

	case jmp:
		const (
			regMemOpcode    = 0xff
			regMemOpcodeNum = 1
			regMemSubOpcode = 4
		)
		op := i.op1
		switch op.kind {
		case operandKindLabel:
			needsLabelResolution = true
			fallthrough
		case operandKindImm32:
			c.EmitByte(0xe9)
			c.Emit4Bytes(op.imm32())
		case operandKindMem:
			m := op.addressMode()
			encodeRegMem(c,
				legacyPrefixesNone,
				regMemOpcode, regMemOpcodeNum,
				regMemSubOpcode, m, rexInfo(0).clearW(),
			)
		case operandKindReg:
			r := op.reg().RealReg()
			encodeRegReg(
				c,
				legacyPrefixesNone,
				regMemOpcode, regMemOpcodeNum,
				regMemSubOpcode,
				regEncodings[r], rexInfo(0).clearW(),
			)
		default:
			panic("BUG: invalid operand kind")
		}

	case jmpIf:
		op := i.op1
		switch op.kind {
		case operandKindLabel:
			needsLabelResolution = true
			fallthrough
		case operandKindImm32:
			c.EmitByte(0x0f)
			c.EmitByte(0x80 | cond(i.u1).encoding())
			c.Emit4Bytes(op.imm32())
		default:
			panic("BUG: invalid operand kind")
		}

	case jmpTableIsland:
		needsLabelResolution = true
		for tc := uint64(0); tc < i.u2; tc++ {
			c.Emit8Bytes(0)
		}

	case exitSequence:
		execCtx := i.op1.reg()
		allocatedAmode := i.op2.addressMode()

		// Restore the RBP, RSP, and return to the Go code:
		*allocatedAmode = amode{
			kindWithShift: uint32(amodeImmReg), base: execCtx,
			imm32: wazevoapi.ExecutionContextOffsetOriginalFramePointer.U32(),
		}
		encodeLoad64(c, allocatedAmode, rbp)
		allocatedAmode.imm32 = wazevoapi.ExecutionContextOffsetOriginalStackPointer.U32()
		encodeLoad64(c, allocatedAmode, rsp)
		encodeRet(c)

	case ud2:
		c.EmitByte(0x0f)
		c.EmitByte(0x0b)

	case call:
		c.EmitByte(0xe8)
		// Meaning that the call target is a function value, and requires relocation.
		c.AddRelocationInfo(ssa.FuncRef(i.u1))
		// Note that this is zero as a placeholder for the call target if it's a function value.
		c.Emit4Bytes(uint32(i.u2))

	case callIndirect:
		op := i.op1

		const opcodeNum = 1
		const opcode = 0xff
		rex := rexInfo(0).clearW()
		switch op.kind {
		case operandKindReg:
			dst := regEncodings[op.reg().RealReg()]
			encodeRegReg(c,
				legacyPrefixesNone,
				opcode, opcodeNum,
				regEnc(2),
				dst,
				rex,
			)
		case operandKindMem:
			m := op.addressMode()
			encodeRegMem(c,
				legacyPrefixesNone,
				opcode, opcodeNum,
				regEnc(2),
				m,
				rex,
			)
		default:
			panic("BUG: invalid operand kind")
		}

	case xchg:
		src, dst := regEncodings[i.op1.reg().RealReg()], i.op2
		size := i.u1

		var rex rexInfo
		var opcode uint32
		lp := legacyPrefixesNone
		switch size {
		case 8:
			opcode = 0x87
			rex = rexInfo(0).setW()
		case 4:
			opcode = 0x87
			rex = rexInfo(0).clearW()
		case 2:
			lp = legacyPrefixes0x66
			opcode = 0x87
			rex = rexInfo(0).clearW()
		case 1:
			opcode = 0x86
			if i.op2.kind == operandKindReg {
				panic("TODO?: xchg on two 1-byte registers")
			}
			// Some destinations must be encoded with REX.R = 1.
			if e := src.encoding(); e >= 4 && e <= 7 {
				rex = rexInfo(0).always()
			}
		default:
			panic(fmt.Sprintf("BUG: invalid size %d: %s", size, i.String()))
		}

		switch dst.kind {
		case operandKindMem:
			m := dst.addressMode()
			encodeRegMem(c, lp, opcode, 1, src, m, rex)
		case operandKindReg:
			r := dst.reg().RealReg()
			encodeRegReg(c, lp, opcode, 1, src, regEncodings[r], rex)
		default:
			panic("BUG: invalid operand kind")
		}

	case lockcmpxchg:
		src, dst := regEncodings[i.op1.reg().RealReg()], i.op2
		size := i.u1

		var rex rexInfo
		var opcode uint32
		lp := legacyPrefixes0xF0 // Lock prefix.
		switch size {
		case 8:
			opcode = 0x0FB1
			rex = rexInfo(0).setW()
		case 4:
			opcode = 0x0FB1
			rex = rexInfo(0).clearW()
		case 2:
			lp = legacyPrefixes0x660xF0 // Legacy prefix + Lock prefix.
			opcode = 0x0FB1
			rex = rexInfo(0).clearW()
		case 1:
			opcode = 0x0FB0
			// Some destinations must be encoded with REX.R = 1.
			if e := src.encoding(); e >= 4 && e <= 7 {
				rex = rexInfo(0).always()
			}
		default:
			panic(fmt.Sprintf("BUG: invalid size %d: %s", size, i.String()))
		}

		switch dst.kind {
		case operandKindMem:
			m := dst.addressMode()
			encodeRegMem(c, lp, opcode, 2, src, m, rex)
		default:
			panic("BUG: invalid operand kind")
		}

	case lockxadd:
		src, dst := regEncodings[i.op1.reg().RealReg()], i.op2
		size := i.u1

		var rex rexInfo
		var opcode uint32
		lp := legacyPrefixes0xF0 // Lock prefix.
		switch size {
		case 8:
			opcode = 0x0FC1
			rex = rexInfo(0).setW()
		case 4:
			opcode = 0x0FC1
			rex = rexInfo(0).clearW()
		case 2:
			lp = legacyPrefixes0x660xF0 // Legacy prefix + Lock prefix.
			opcode = 0x0FC1
			rex = rexInfo(0).clearW()
		case 1:
			opcode = 0x0FC0
			// Some destinations must be encoded with REX.R = 1.
			if e := src.encoding(); e >= 4 && e <= 7 {
				rex = rexInfo(0).always()
			}
		default:
			panic(fmt.Sprintf("BUG: invalid size %d: %s", size, i.String()))
		}

		switch dst.kind {
		case operandKindMem:
			m := dst.addressMode()
			encodeRegMem(c, lp, opcode, 2, src, m, rex)
		default:
			panic("BUG: invalid operand kind")
		}

	case zeros:
		r := i.op2.reg()
		if r.RegType() == regalloc.RegTypeInt {
			i.asAluRmiR(aluRmiROpcodeXor, newOperandReg(r), r, true)
		} else {
			i.asXmmRmR(sseOpcodePxor, newOperandReg(r), r)
		}
		i.encode(c)

	case mfence:
		// https://www.felixcloutier.com/x86/mfence
		c.EmitByte(0x0f)
		c.EmitByte(0xae)
		c.EmitByte(0xf0)

	default:
		panic(fmt.Sprintf("TODO: %v", i.kind))
	}
	return
}

func encodeLoad64(c backend.Compiler, m *amode, rd regalloc.RealReg) {
	dst := regEncodings[rd]
	encodeRegMem(c, legacyPrefixesNone, 0x8b, 1, dst, m, rexInfo(0).setW())
}

func encodeRet(c backend.Compiler) {
	c.EmitByte(0xc3)
}

func encodeEncEnc(
	c backend.Compiler,
	legPrefixes legacyPrefixes,
	opcodes uint32,
	opcodeNum uint32,
	r uint8,
	rm uint8,
	rex rexInfo,
) {
	legPrefixes.encode(c)
	rex.encode(c, r>>3, rm>>3)

	for opcodeNum > 0 {
		opcodeNum--
		c.EmitByte(byte((opcodes >> (opcodeNum << 3)) & 0xff))
	}
	c.EmitByte(encodeModRM(3, r&7, rm&7))
}

func encodeRegReg(
	c backend.Compiler,
	legPrefixes legacyPrefixes,
	opcodes uint32,
	opcodeNum uint32,
	r regEnc,
	rm regEnc,
	rex rexInfo,
) {
	encodeEncEnc(c, legPrefixes, opcodes, opcodeNum, uint8(r), uint8(rm), rex)
}

func encodeModRM(mod byte, reg byte, rm byte) byte {
	return mod<<6 | reg<<3 | rm
}

func encodeSIB(shift byte, encIndex byte, encBase byte) byte {
	return shift<<6 | encIndex<<3 | encBase
}

func encodeRegMem(
	c backend.Compiler, legPrefixes legacyPrefixes, opcodes uint32, opcodeNum uint32, r regEnc, m *amode, rex rexInfo,
) (needsLabelResolution bool) {
	needsLabelResolution = encodeEncMem(c, legPrefixes, opcodes, opcodeNum, uint8(r), m, rex)
	return
}

func encodeEncMem(
	c backend.Compiler, legPrefixes legacyPrefixes, opcodes uint32, opcodeNum uint32, r uint8, m *amode, rex rexInfo,
) (needsLabelResolution bool) {
	legPrefixes.encode(c)

	const (
		modNoDisplacement    = 0b00
		modShortDisplacement = 0b01
		modLongDisplacement  = 0b10

		useSBI = 4 // the encoding of rsp or r12 register.
	)

	switch m.kind() {
	case amodeImmReg, amodeImmRBP:
		base := m.base.RealReg()
		baseEnc := regEncodings[base]

		rex.encode(c, regRexBit(r), baseEnc.rexBit())

		for opcodeNum > 0 {
			opcodeNum--
			c.EmitByte(byte((opcodes >> (opcodeNum << 3)) & 0xff))
		}

		// SIB byte is the last byte of the memory encoding before the displacement
		const sibByte = 0x24 // == encodeSIB(0, 4, 4)

		immZero, baseRbp, baseR13 := m.imm32 == 0, base == rbp, base == r13
		short := lower8willSignExtendTo32(m.imm32)
		rspOrR12 := base == rsp || base == r12

		if immZero && !baseRbp && !baseR13 { // rbp or r13 can't be used as base for without displacement encoding.
			c.EmitByte(encodeModRM(modNoDisplacement, regEncoding(r), baseEnc.encoding()))
			if rspOrR12 {
				c.EmitByte(sibByte)
			}
		} else if short { // Note: this includes the case where m.imm32 == 0 && base == rbp || base == r13.
			c.EmitByte(encodeModRM(modShortDisplacement, regEncoding(r), baseEnc.encoding()))
			if rspOrR12 {
				c.EmitByte(sibByte)
			}
			c.EmitByte(byte(m.imm32))
		} else {
			c.EmitByte(encodeModRM(modLongDisplacement, regEncoding(r), baseEnc.encoding()))
			if rspOrR12 {
				c.EmitByte(sibByte)
			}
			c.Emit4Bytes(m.imm32)
		}

	case amodeRegRegShift:
		base := m.base.RealReg()
		baseEnc := regEncodings[base]
		index := m.index.RealReg()
		indexEnc := regEncodings[index]

		if index == rsp {
			panic("BUG: rsp can't be used as index of addressing mode")
		}

		rex.encodeForIndex(c, regEnc(r), indexEnc, baseEnc)

		for opcodeNum > 0 {
			opcodeNum--
			c.EmitByte(byte((opcodes >> (opcodeNum << 3)) & 0xff))
		}

		immZero, baseRbp, baseR13 := m.imm32 == 0, base == rbp, base == r13
		if immZero && !baseRbp && !baseR13 { // rbp or r13 can't be used as base for without displacement encoding. (curious why? because it's interpreted as RIP relative addressing).
			c.EmitByte(encodeModRM(modNoDisplacement, regEncoding(r), useSBI))
			c.EmitByte(encodeSIB(m.shift(), indexEnc.encoding(), baseEnc.encoding()))
		} else if lower8willSignExtendTo32(m.imm32) {
			c.EmitByte(encodeModRM(modShortDisplacement, regEncoding(r), useSBI))
			c.EmitByte(encodeSIB(m.shift(), indexEnc.encoding(), baseEnc.encoding()))
			c.EmitByte(byte(m.imm32))
		} else {
			c.EmitByte(encodeModRM(modLongDisplacement, regEncoding(r), useSBI))
			c.EmitByte(encodeSIB(m.shift(), indexEnc.encoding(), baseEnc.encoding()))
			c.Emit4Bytes(m.imm32)
		}

	case amodeRipRel:
		rex.encode(c, regRexBit(r), 0)
		for opcodeNum > 0 {
			opcodeNum--
			c.EmitByte(byte((opcodes >> (opcodeNum << 3)) & 0xff))
		}

		// Indicate "LEAQ [RIP + 32bit displacement].
		// https://wiki.osdev.org/X86-64_Instruction_Encoding#32.2F64-bit_addressing
		c.EmitByte(encodeModRM(0b00, regEncoding(r), 0b101))

		// This will be resolved later, so we just emit a placeholder.
		needsLabelResolution = true
		c.Emit4Bytes(0)

	default:
		panic("BUG: invalid addressing mode")
	}
	return
}

const (
	rexEncodingDefault byte = 0x40
	rexEncodingW            = rexEncodingDefault | 0x08
)

// rexInfo is a bit set to indicate:
//
//	0x01: W bit must be cleared.
//	0x02: REX prefix must be emitted.
type rexInfo byte

func (ri rexInfo) setW() rexInfo {
	return ri | 0x01
}

func (ri rexInfo) clearW() rexInfo {
	return ri & 0x02
}

func (ri rexInfo) always() rexInfo {
	return ri | 0x02
}

func (ri rexInfo) notAlways() rexInfo { //nolint
	return ri & 0x01
}

func (ri rexInfo) encode(c backend.Compiler, r uint8, b uint8) {
	var w byte = 0
	if ri&0x01 != 0 {
		w = 0x01
	}
	rex := rexEncodingDefault | w<<3 | r<<2 | b
	if rex != rexEncodingDefault || ri&0x02 != 0 {
		c.EmitByte(rex)
	}
}

func (ri rexInfo) encodeForIndex(c backend.Compiler, encR regEnc, encIndex regEnc, encBase regEnc) {
	var w byte = 0
	if ri&0x01 != 0 {
		w = 0x01
	}
	r := encR.rexBit()
	x := encIndex.rexBit()
	b := encBase.rexBit()
	rex := byte(0x40) | w<<3 | r<<2 | x<<1 | b
	if rex != 0x40 || ri&0x02 != 0 {
		c.EmitByte(rex)
	}
}

type regEnc byte

func (r regEnc) rexBit() byte {
	return regRexBit(byte(r))
}

func (r regEnc) encoding() byte {
	return regEncoding(byte(r))
}

func regRexBit(r byte) byte {
	return r >> 3
}

func regEncoding(r byte) byte {
	return r & 0x07
}

var regEncodings = [...]regEnc{
	rax:   0b000,
	rcx:   0b001,
	rdx:   0b010,
	rbx:   0b011,
	rsp:   0b100,
	rbp:   0b101,
	rsi:   0b110,
	rdi:   0b111,
	r8:    0b1000,
	r9:    0b1001,
	r10:   0b1010,
	r11:   0b1011,
	r12:   0b1100,
	r13:   0b1101,
	r14:   0b1110,
	r15:   0b1111,
	xmm0:  0b000,
	xmm1:  0b001,
	xmm2:  0b010,
	xmm3:  0b011,
	xmm4:  0b100,
	xmm5:  0b101,
	xmm6:  0b110,
	xmm7:  0b111,
	xmm8:  0b1000,
	xmm9:  0b1001,
	xmm10: 0b1010,
	xmm11: 0b1011,
	xmm12: 0b1100,
	xmm13: 0b1101,
	xmm14: 0b1110,
	xmm15: 0b1111,
}

type legacyPrefixes byte

const (
	legacyPrefixesNone legacyPrefixes = iota
	legacyPrefixes0x66
	legacyPrefixes0xF0
	legacyPrefixes0x660xF0
	legacyPrefixes0xF2
	legacyPrefixes0xF3
)

func (p legacyPrefixes) encode(c backend.Compiler) {
	switch p {
	case legacyPrefixesNone:
	case legacyPrefixes0x66:
		c.EmitByte(0x66)
	case legacyPrefixes0xF0:
		c.EmitByte(0xf0)
	case legacyPrefixes0x660xF0:
		c.EmitByte(0x66)
		c.EmitByte(0xf0)
	case legacyPrefixes0xF2:
		c.EmitByte(0xf2)
	case legacyPrefixes0xF3:
		c.EmitByte(0xf3)
	default:
		panic("BUG: invalid legacy prefix")
	}
}

func lower32willSignExtendTo64(x uint64) bool {
	xs := int64(x)
	return xs == int64(uint64(int32(xs)))
}

func lower8willSignExtendTo32(x uint32) bool {
	xs := int32(x)
	return xs == ((xs << 24) >> 24)
}
