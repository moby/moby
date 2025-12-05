package arm64

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
)

const (
	// trampolineCallSize is the size of the trampoline instruction sequence for each function in an island.
	trampolineCallSize = 4*4 + 4 // Four instructions + 32-bit immediate.

	// Unconditional branch offset is encoded as divided by 4 in imm26.
	// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/BL--Branch-with-Link-?lang=en

	maxUnconditionalBranchOffset = maxSignedInt26 * 4
	minUnconditionalBranchOffset = minSignedInt26 * 4

	// trampolineIslandInterval is the range of the trampoline island.
	// Half of the range is used for the trampoline island, and the other half is used for the function.
	trampolineIslandInterval = (maxUnconditionalBranchOffset - 1) / 2

	// maxNumFunctions explicitly specifies the maximum number of functions that can be allowed in a single executable.
	maxNumFunctions = trampolineIslandInterval >> 6

	// maxFunctionExecutableSize is the maximum size of a function that can exist in a trampoline island.
	// Conservatively set to 1/4 of the trampoline island interval.
	maxFunctionExecutableSize = trampolineIslandInterval >> 2
)

// CallTrampolineIslandInfo implements backend.Machine CallTrampolineIslandInfo.
func (m *machine) CallTrampolineIslandInfo(numFunctions int) (interval, size int, err error) {
	if numFunctions > maxNumFunctions {
		return 0, 0, fmt.Errorf("too many functions: %d > %d", numFunctions, maxNumFunctions)
	}
	return trampolineIslandInterval, trampolineCallSize * numFunctions, nil
}

// ResolveRelocations implements backend.Machine ResolveRelocations.
func (m *machine) ResolveRelocations(
	refToBinaryOffset []int,
	importedFns int,
	executable []byte,
	relocations []backend.RelocationInfo,
	callTrampolineIslandOffsets []int,
) {
	for _, islandOffset := range callTrampolineIslandOffsets {
		encodeCallTrampolineIsland(refToBinaryOffset, importedFns, islandOffset, executable)
	}

	for _, r := range relocations {
		instrOffset := r.Offset
		calleeFnOffset := refToBinaryOffset[r.FuncRef]
		diff := int64(calleeFnOffset) - (instrOffset)
		// Check if the diff is within the range of the branch instruction.
		if diff < minUnconditionalBranchOffset || diff > maxUnconditionalBranchOffset {
			// Find the near trampoline island from callTrampolineIslandOffsets.
			islandOffset := searchTrampolineIsland(callTrampolineIslandOffsets, int(instrOffset))
			islandTargetOffset := islandOffset + trampolineCallSize*int(r.FuncRef)
			diff = int64(islandTargetOffset) - (instrOffset)
			if diff < minUnconditionalBranchOffset || diff > maxUnconditionalBranchOffset {
				panic("BUG in trampoline placement")
			}
		}
		binary.LittleEndian.PutUint32(executable[instrOffset:instrOffset+4], encodeUnconditionalBranch(true, diff))
	}
}

// encodeCallTrampolineIsland encodes a trampoline island for the given functions.
// Each island consists of a trampoline instruction sequence for each function.
// Each trampoline instruction sequence consists of 4 instructions + 32-bit immediate.
func encodeCallTrampolineIsland(refToBinaryOffset []int, importedFns int, islandOffset int, executable []byte) {
	// We skip the imported functions: they don't need trampolines
	// and are not accounted for.
	binaryOffsets := refToBinaryOffset[importedFns:]

	for i := 0; i < len(binaryOffsets); i++ {
		trampolineOffset := islandOffset + trampolineCallSize*i

		fnOffset := binaryOffsets[i]
		diff := fnOffset - (trampolineOffset + 16)
		if diff > math.MaxInt32 || diff < math.MinInt32 {
			// This case even amd64 can't handle. 4GB is too big.
			panic("too big binary")
		}

		// The tmpReg, tmpReg2 is safe to overwrite (in fact any caller-saved register is safe to use).
		tmpReg, tmpReg2 := regNumberInEncoding[tmpRegVReg.RealReg()], regNumberInEncoding[x11]

		// adr tmpReg, PC+16: load the address of #diff into tmpReg.
		binary.LittleEndian.PutUint32(executable[trampolineOffset:], encodeAdr(tmpReg, 16))
		// ldrsw tmpReg2, [tmpReg]: Load #diff into tmpReg2.
		binary.LittleEndian.PutUint32(executable[trampolineOffset+4:],
			encodeLoadOrStore(sLoad32, tmpReg2, addressMode{kind: addressModeKindRegUnsignedImm12, rn: tmpRegVReg}))
		// add tmpReg, tmpReg2, tmpReg: add #diff to the address of #diff, getting the absolute address of the function.
		binary.LittleEndian.PutUint32(executable[trampolineOffset+8:],
			encodeAluRRR(aluOpAdd, tmpReg, tmpReg, tmpReg2, true, false))
		// br tmpReg: branch to the function without overwriting the link register.
		binary.LittleEndian.PutUint32(executable[trampolineOffset+12:], encodeUnconditionalBranchReg(tmpReg, false))
		// #diff
		binary.LittleEndian.PutUint32(executable[trampolineOffset+16:], uint32(diff))
	}
}

// searchTrampolineIsland finds the nearest trampoline island from callTrampolineIslandOffsets.
// Note that even if the offset is in the middle of two islands, it returns the latter one.
// That is ok because the island is always placed in the middle of the range.
//
// precondition: callTrampolineIslandOffsets is sorted in ascending order.
func searchTrampolineIsland(callTrampolineIslandOffsets []int, offset int) int {
	l := len(callTrampolineIslandOffsets)
	n := sort.Search(l, func(i int) bool {
		return callTrampolineIslandOffsets[i] >= offset
	})
	if n == l {
		n = l - 1
	}
	return callTrampolineIslandOffsets[n]
}
