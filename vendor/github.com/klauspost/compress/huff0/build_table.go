package huff0

import "errors"

// BuildCTable builds a Huffman compression table from a precomputed symbol
// histogram and installs it as the previous (reuse) table on s.
//
// After this call:
//   - EstimateSize/CanUseTable can probe the table against other histograms.
//   - Compress1X/Compress4X with Reuse = ReusePolicyMust will encode without
//     emitting a new table header.
//   - TransferCTable can hand the table to a sibling Scratch.
//
// count[i] is the number of occurrences of symbol i. The histogram must have
// at least 2 distinct non-zero symbols; ErrUseRLE is returned for a single
// symbol and an error is returned for an empty histogram.
func (s *Scratch) BuildCTable(count *[256]uint32) error {
	if s == nil {
		return errors.New("huff0: BuildCTable on nil Scratch")
	}
	if count == nil {
		return errors.New("huff0: nil count passed to BuildCTable")
	}
	var err error
	s, err = s.prepare(nil)
	if err != nil {
		return err
	}
	s.count = *count
	var total, maxCount int
	var symLen uint16
	for i, v := range s.count {
		total += int(v)
		if int(v) > maxCount {
			maxCount = int(v)
		}
		if v != 0 {
			symLen = uint16(i) + 1
		}
	}
	if total == 0 {
		return errors.New("huff0: empty histogram")
	}
	if symLen < 2 || maxCount == total {
		return ErrUseRLE
	}
	// huff0's internal rank table assumes total ≤ BlockSizeMax (it uses
	// highBit32(count+1) + 1 as a rank index into a fixed-size array).
	// Histograms summed across multiple blocks can exceed that; scale the
	// counts down preserving the distribution. Non-zero entries round up so
	// rare symbols stay representable.
	if total > BlockSizeMax {
		shift := uint(0)
		for total>>shift > BlockSizeMax {
			shift++
		}
		round := uint32(1<<shift) - 1
		var newTotal, newMax int
		for i, v := range s.count {
			if v == 0 {
				continue
			}
			scaled := (v + round) >> shift
			if scaled == 0 {
				scaled = 1
			}
			s.count[i] = scaled
			newTotal += int(scaled)
			if int(scaled) > newMax {
				newMax = int(scaled)
			}
		}
		total = newTotal
		maxCount = newMax
		if maxCount == total {
			return ErrUseRLE
		}
	}
	s.symbolLen = symLen
	s.maxCount = maxCount
	s.srcLen = total
	if err := s.buildCTable(); err != nil {
		return err
	}
	if cap(s.prevTable) < len(s.cTable) {
		s.prevTable = make(cTable, 0, maxSymbolValue+1)
	}
	s.prevTable = s.prevTable[:len(s.cTable)]
	copy(s.prevTable, s.cTable)
	s.prevTableLog = s.actualTableLog
	// Force the next Compress* to recount from real input.
	s.clearCount = true
	s.maxCount = 0
	return nil
}

// EstimateSize returns an estimated compressed payload size in bytes for the
// supplied histogram using the table currently stored in prevTable. It returns
// -1 when the table cannot encode every non-zero symbol of hist (i.e. when
// CanUseTable would return false). The estimate excludes the table header.
func (s *Scratch) EstimateSize(hist *[256]uint32) int {
	if s == nil || hist == nil || len(s.prevTable) == 0 {
		return -1
	}
	pt := s.prevTable
	nbBits := uint32(7)
	for i, v := range hist {
		if v == 0 {
			continue
		}
		if i >= len(pt) || pt[i].nBits == 0 {
			return -1
		}
		nbBits += uint32(pt[i].nBits) * v
	}
	return int(nbBits >> 3)
}

// CanUseTable reports whether the table in prevTable can encode every
// non-zero symbol present in hist.
func (s *Scratch) CanUseTable(hist *[256]uint32) bool {
	if s == nil || hist == nil || len(s.prevTable) == 0 {
		return false
	}
	pt := s.prevTable
	for i, v := range hist {
		if v == 0 {
			continue
		}
		if i >= len(pt) || pt[i].nBits == 0 {
			return false
		}
	}
	return true
}

// AppendTable serializes the table currently stored in prevTable (e.g. as
// installed by BuildCTable or carried over from a previous Compress call)
// into a self-delimiting zstd-style header and appends it to dst. The
// returned slice can be parsed back by ReadTable.
func (s *Scratch) AppendTable(dst []byte) ([]byte, error) {
	if s == nil || len(s.prevTable) == 0 {
		return dst, errors.New("huff0: AppendTable with empty table")
	}
	// cTable.write reads s.actualTableLog, s.symbolLen, s.huffWeight, s.fse
	// and writes into s.Out. Save/restore Out so we don't disturb in-flight
	// compression buffers.
	saveOut := s.Out
	saveTL := s.actualTableLog
	saveSL := s.symbolLen
	if s.fse == nil {
		// Lazily init in case AppendTable is called on a fresh Scratch.
		if _, err := s.prepare(nil); err != nil {
			return dst, err
		}
		saveOut = s.Out
	}
	s.Out = s.Out[:0]
	s.actualTableLog = s.prevTableLog
	s.symbolLen = uint16(len(s.prevTable))
	if err := s.prevTable.write(s); err != nil {
		s.Out, s.actualTableLog, s.symbolLen = saveOut, saveTL, saveSL
		return dst, err
	}
	dst = append(dst, s.Out...)
	s.Out, s.actualTableLog, s.symbolLen = saveOut, saveTL, saveSL
	return dst, nil
}
