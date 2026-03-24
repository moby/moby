package wasmdebug

import (
	"debug/dwarf"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// DWARFLines is used to retrieve source code line information from the DWARF data.
type DWARFLines struct {
	// d is created by DWARF custom sections.
	d *dwarf.Data
	// linesPerEntry maps dwarf.Offset for dwarf.Entry to the list of lines contained by the entry.
	// The value is sorted in the increasing order by the address.
	linesPerEntry map[dwarf.Offset][]line
	mux           sync.Mutex
}

type line struct {
	addr uint64
	pos  dwarf.LineReaderPos
}

// NewDWARFLines returns DWARFLines for the given *dwarf.Data.
func NewDWARFLines(d *dwarf.Data) *DWARFLines {
	if d == nil {
		return nil
	}
	return &DWARFLines{d: d, linesPerEntry: map[dwarf.Offset][]line{}}
}

// isTombstoneAddr returns true if the given address is invalid a.k.a tombstone address which was made no longer valid
// by linker. According to the DWARF spec[1], the value is encoded as 0xffffffff for Wasm (as 32-bit target),
// but some tools encode it either in -1, -2 [2] or 1<<32 (This might not be by tools, but by debug/dwarf package's bug).
//
// [1] https://dwarfstd.org/issues/200609.1.html
// [2] https://github.com/WebAssembly/binaryen/blob/97178d08d4a20d2a5e3a6be813fc6a7079ef86e1/src/wasm/wasm-debug.cpp#L651-L660
// [3] https://reviews.llvm.org/D81784
func isTombstoneAddr(addr uint64) bool {
	addr32 := int32(addr)
	return addr32 == -1 || addr32 == -2 ||
		addr32 == 0 // This covers 1 <<32.
}

// Line returns the line information for the given instructionOffset which is an offset in
// the code section of the original Wasm binary. Returns empty string if the info is not found.
func (d *DWARFLines) Line(instructionOffset uint64) (ret []string) {
	if d == nil {
		return
	}

	// DWARFLines is created per Wasm binary, so there's a possibility that multiple instances
	// created from a same binary face runtime error at the same time, and that results in
	// concurrent access to this function.
	d.mux.Lock()
	defer d.mux.Unlock()

	r := d.d.Reader()

	var inlinedRoutines []*dwarf.Entry
	var cu *dwarf.Entry
	var inlinedDone bool
entry:
	for {
		ent, err := r.Next()
		if err != nil || ent == nil {
			break
		}

		// If we already found the compilation unit and relevant inlined routines, we can stop searching entries.
		if cu != nil && inlinedDone {
			break
		}

		switch ent.Tag {
		case dwarf.TagCompileUnit, dwarf.TagInlinedSubroutine:
		default:
			// Only CompileUnit and InlinedSubroutines are relevant.
			continue
		}

		// Check if the entry spans the range which contains the target instruction.
		ranges, err := d.d.Ranges(ent)
		if err != nil {
			continue
		}
		for _, pcs := range ranges {
			start, end := pcs[0], pcs[1]
			if isTombstoneAddr(start) || isTombstoneAddr(end) {
				continue
			}
			if start <= instructionOffset && instructionOffset < end {
				switch ent.Tag {
				case dwarf.TagCompileUnit:
					cu = ent
				case dwarf.TagInlinedSubroutine:
					inlinedRoutines = append(inlinedRoutines, ent)
					// Search inlined subroutines until all the children.
					inlinedDone = !ent.Children
					// Not that "children" in the DWARF spec is defined as the next entry to this entry.
					// See "2.3 Relationship of Debugging Information Entries" in https://dwarfstd.org/doc/DWARF4.pdf
				}
				continue entry
			}
		}
	}

	// If the relevant compilation unit is not found, nothing we can do with this DWARF info.
	if cu == nil {
		return
	}

	lineReader, err := d.d.LineReader(cu)
	if err != nil || lineReader == nil {
		return
	}
	var lines []line
	var ok bool
	var le dwarf.LineEntry
	// Get the lines inside the entry.
	if lines, ok = d.linesPerEntry[cu.Offset]; !ok {
		// If not found, we create the list of lines by reading all the LineEntries in the Entry.
		//
		// Note that the dwarf.LineEntry.SeekPC API shouldn't be used because the Go's dwarf package assumes that
		// all the line entries in an Entry are sorted in increasing order which *might not* be true
		// for some languages. Such order requirement is not a part of DWARF specification,
		// and in fact Zig language tends to emit interleaved line information.
		//
		// Thus, here we read all line entries here, and sort them in the increasing order wrt addresses.
		for {
			pos := lineReader.Tell()
			err = lineReader.Next(&le)
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return
			}
			// TODO: Maybe we should ignore tombstone addresses by using isTombstoneAddr,
			//  but not sure if that would be an issue in practice.
			lines = append(lines, line{addr: le.Address, pos: pos})
		}
		sort.Slice(lines, func(i, j int) bool { return lines[i].addr < lines[j].addr })
		d.linesPerEntry[cu.Offset] = lines // Caches for the future inquiries for the same Entry.
	}

	// Now we have the lines for this entry. We can find the corresponding source line for instructionOffset
	// via binary search on the list.
	n := len(lines)
	index := sort.Search(n, func(i int) bool { return lines[i].addr >= instructionOffset })

	if index == n { // This case the address is not found. See the doc sort.Search.
		return
	}

	ln := lines[index]
	if ln.addr != instructionOffset {
		// If the address doesn't match exactly, the previous entry is the one that contains the instruction.
		// That can happen anytime as the DWARF spec allows it, and other tools can handle it in this way conventionally
		// https://github.com/gimli-rs/addr2line/blob/3a2dbaf84551a06a429f26e9c96071bb409b371f/src/lib.rs#L236-L242
		// https://github.com/kateinoigakukun/wasminspect/blob/f29f052f1b03104da9f702508ac0c1bbc3530ae4/crates/debugger/src/dwarf/mod.rs#L453-L459
		if index-1 < 0 {
			return
		}
		ln = lines[index-1]
	}

	// Advance the line reader for the found position.
	lineReader.Seek(ln.pos)
	err = lineReader.Next(&le)
	if err != nil {
		// If we reach this block, that means there's a bug in the []line creation logic above.
		panic("BUG: stored dwarf.LineReaderPos is invalid")
	}

	// In the inlined case, the line info is the innermost inlined function call.
	inlined := len(inlinedRoutines) != 0
	prefix := fmt.Sprintf("%#x: ", instructionOffset)
	ret = append(ret, formatLine(prefix, le.File.Name, int64(le.Line), int64(le.Column), inlined))

	if inlined {
		prefix = strings.Repeat(" ", len(prefix))
		files := lineReader.Files()
		// inlinedRoutines contain the inlined call information in the reverse order (children is higher than parent),
		// so we traverse the reverse order and emit the inlined calls.
		for i := len(inlinedRoutines) - 1; i >= 0; i-- {
			inlined := inlinedRoutines[i]
			fileIndex, ok := inlined.Val(dwarf.AttrCallFile).(int64)
			if !ok {
				return
			} else if fileIndex >= int64(len(files)) {
				// This in theory shouldn't happen according to the spec, but guard against ill-formed DWARF info.
				return
			}
			fileName := files[fileIndex]
			line, _ := inlined.Val(dwarf.AttrCallLine).(int64)
			col, _ := inlined.Val(dwarf.AttrCallColumn).(int64)
			ret = append(ret, formatLine(prefix, fileName.Name, line, col,
				// Last one is the origin of the inlined function calls.
				i != 0))
		}
	}
	return
}

func formatLine(prefix, fileName string, line, col int64, inlined bool) string {
	builder := strings.Builder{}
	builder.WriteString(prefix)
	builder.WriteString(fileName)

	if line != 0 {
		builder.WriteString(fmt.Sprintf(":%d", line))
		if col != 0 {
			builder.WriteString(fmt.Sprintf(":%d", col))
		}
	}

	if inlined {
		builder.WriteString(" (inlined)")
	}
	return builder.String()
}
