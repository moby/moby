package bitseq

import (
	"testing"
)

func TestSequenceGetAvailableBit(t *testing.T) {
	input := []struct {
		head    *Sequence
		bytePos int
		bitPos  int
	}{
		{&Sequence{Block: 0x0, Count: 0}, -1, -1},
		{&Sequence{Block: 0x0, Count: 1}, 0, 0},
		{&Sequence{Block: 0x0, Count: 100}, 0, 0},

		{&Sequence{Block: 0x80000000, Count: 0}, -1, -1},
		{&Sequence{Block: 0x80000000, Count: 1}, 0, 1},
		{&Sequence{Block: 0x80000000, Count: 100}, 0, 1},

		{&Sequence{Block: 0xFF000000, Count: 0}, -1, -1},
		{&Sequence{Block: 0xFF000000, Count: 1}, 1, 0},
		{&Sequence{Block: 0xFF000000, Count: 100}, 1, 0},

		{&Sequence{Block: 0xFF800000, Count: 0}, -1, -1},
		{&Sequence{Block: 0xFF800000, Count: 1}, 1, 1},
		{&Sequence{Block: 0xFF800000, Count: 100}, 1, 1},

		{&Sequence{Block: 0xFFC0FF00, Count: 0}, -1, -1},
		{&Sequence{Block: 0xFFC0FF00, Count: 1}, 1, 2},
		{&Sequence{Block: 0xFFC0FF00, Count: 100}, 1, 2},

		{&Sequence{Block: 0xFFE0FF00, Count: 0}, -1, -1},
		{&Sequence{Block: 0xFFE0FF00, Count: 1}, 1, 3},
		{&Sequence{Block: 0xFFE0FF00, Count: 100}, 1, 3},

		{&Sequence{Block: 0xFFFEFF00, Count: 0}, -1, -1},
		{&Sequence{Block: 0xFFFEFF00, Count: 1}, 1, 7},
		{&Sequence{Block: 0xFFFEFF00, Count: 100}, 1, 7},

		{&Sequence{Block: 0xFFFFC0FF, Count: 0}, -1, -1},
		{&Sequence{Block: 0xFFFFC0FF, Count: 1}, 2, 2},
		{&Sequence{Block: 0xFFFFC0FF, Count: 100}, 2, 2},

		{&Sequence{Block: 0xFFFFFF00, Count: 0}, -1, -1},
		{&Sequence{Block: 0xFFFFFF00, Count: 1}, 3, 0},
		{&Sequence{Block: 0xFFFFFF00, Count: 100}, 3, 0},

		{&Sequence{Block: 0xFFFFFFFE, Count: 0}, -1, -1},
		{&Sequence{Block: 0xFFFFFFFE, Count: 1}, 3, 7},
		{&Sequence{Block: 0xFFFFFFFE, Count: 100}, 3, 7},

		{&Sequence{Block: 0xFFFFFFFF, Count: 0}, -1, -1},
		{&Sequence{Block: 0xFFFFFFFF, Count: 1}, -1, -1},
		{&Sequence{Block: 0xFFFFFFFF, Count: 100}, -1, -1},
	}

	for n, i := range input {
		b, bb := i.head.GetAvailableBit()
		if b != i.bytePos || bb != i.bitPos {
			t.Fatalf("Error in Sequence.getAvailableBit() (%d).\nExp: (%d, %d)\nGot: (%d, %d),", n, i.bytePos, i.bitPos, b, bb)
		}
	}
}

func TestSequenceEqual(t *testing.T) {
	input := []struct {
		first    *Sequence
		second   *Sequence
		areEqual bool
	}{
		{&Sequence{Block: 0x0, Count: 8, Next: nil}, &Sequence{Block: 0x0, Count: 8}, true},
		{&Sequence{Block: 0x0, Count: 0, Next: nil}, &Sequence{Block: 0x0, Count: 0}, true},
		{&Sequence{Block: 0x0, Count: 2, Next: nil}, &Sequence{Block: 0x0, Count: 1, Next: &Sequence{Block: 0x0, Count: 1}}, false},
		{&Sequence{Block: 0x0, Count: 2, Next: &Sequence{Block: 0x1, Count: 1}}, &Sequence{Block: 0x0, Count: 2}, false},

		{&Sequence{Block: 0x12345678, Count: 8, Next: nil}, &Sequence{Block: 0x12345678, Count: 8}, true},
		{&Sequence{Block: 0x12345678, Count: 8, Next: nil}, &Sequence{Block: 0x12345678, Count: 9}, false},
		{&Sequence{Block: 0x12345678, Count: 1, Next: &Sequence{Block: 0XFFFFFFFF, Count: 1}}, &Sequence{Block: 0x12345678, Count: 1}, false},
		{&Sequence{Block: 0x12345678, Count: 1}, &Sequence{Block: 0x12345678, Count: 1, Next: &Sequence{Block: 0XFFFFFFFF, Count: 1}}, false},
	}

	for n, i := range input {
		if i.areEqual != i.first.Equal(i.second) {
			t.Fatalf("Error in Sequence.Equal() (%d).\nExp: %t\nGot: %t,", n, i.areEqual, !i.areEqual)
		}
	}
}

func TestSequenceCopy(t *testing.T) {
	s := &Sequence{
		Block: 0x0,
		Count: 8,
		Next: &Sequence{
			Block: 0x0,
			Count: 8,
			Next: &Sequence{
				Block: 0x0,
				Count: 0,
				Next: &Sequence{
					Block: 0x0,
					Count: 0,
					Next: &Sequence{
						Block: 0x0,
						Count: 2,
						Next: &Sequence{
							Block: 0x0,
							Count: 1,
							Next: &Sequence{
								Block: 0x0,
								Count: 1,
								Next: &Sequence{
									Block: 0x0,
									Count: 2,
									Next: &Sequence{
										Block: 0x1,
										Count: 1,
										Next: &Sequence{
											Block: 0x0,
											Count: 2,
											Next:  nil,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	n := s.GetCopy()
	if !s.Equal(n) {
		t.Fatalf("copy of s failed")
	}
	if n == s {
		t.Fatalf("not true copy of s")
	}
}

func TestGetFirstAvailable(t *testing.T) {
	input := []struct {
		mask    *Sequence
		bytePos int
		bitPos  int
	}{
		{&Sequence{Block: 0xffffffff, Count: 2048}, -1, -1},
		{&Sequence{Block: 0x0, Count: 8}, 0, 0},
		{&Sequence{Block: 0x80000000, Count: 8}, 0, 1},
		{&Sequence{Block: 0xC0000000, Count: 8}, 0, 2},
		{&Sequence{Block: 0xE0000000, Count: 8}, 0, 3},
		{&Sequence{Block: 0xF0000000, Count: 8}, 0, 4},
		{&Sequence{Block: 0xF8000000, Count: 8}, 0, 5},
		{&Sequence{Block: 0xFC000000, Count: 8}, 0, 6},
		{&Sequence{Block: 0xFE000000, Count: 8}, 0, 7},

		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0x00000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 0},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0x80000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 1},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xC0000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 2},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xE0000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 3},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xF0000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 4},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xF8000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 5},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFC000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 6},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFE000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 7},

		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFF000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 0},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFF800000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 1},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFC00000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 2},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFE00000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 3},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFF00000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 4},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFF80000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 5},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFFC0000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 6},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFFE0000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 7},

		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xfffffffe, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 7, 7},

		{&Sequence{Block: 0xffffffff, Count: 2, Next: &Sequence{Block: 0x0, Count: 6}}, 8, 0},
	}

	for n, i := range input {
		bytePos, bitPos, _ := GetFirstAvailable(i.mask)
		if bytePos != i.bytePos || bitPos != i.bitPos {
			t.Fatalf("Error in (%d) getFirstAvailable(). Expected (%d, %d). Got (%d, %d)", n, i.bytePos, i.bitPos, bytePos, bitPos)
		}
	}
}

func TestFindSequence(t *testing.T) {
	input := []struct {
		head           *Sequence
		bytePos        int
		precBlocks     uint32
		inBlockBytePos int
	}{
		{&Sequence{Block: 0xffffffff, Count: 0}, 0, 0, -1},
		{&Sequence{Block: 0xffffffff, Count: 0}, 31, 0, -1},
		{&Sequence{Block: 0xffffffff, Count: 0}, 100, 0, -1},

		{&Sequence{Block: 0x0, Count: 1}, 0, 0, 0},
		{&Sequence{Block: 0x0, Count: 1}, 1, 0, 1},
		{&Sequence{Block: 0x0, Count: 1}, 31, 0, -1},
		{&Sequence{Block: 0x0, Count: 1}, 60, 0, -1},

		{&Sequence{Block: 0xffffffff, Count: 10}, 0, 0, 0},
		{&Sequence{Block: 0xffffffff, Count: 10}, 3, 0, 3},
		{&Sequence{Block: 0xffffffff, Count: 10}, 4, 1, 0},
		{&Sequence{Block: 0xffffffff, Count: 10}, 7, 1, 3},
		{&Sequence{Block: 0xffffffff, Count: 10}, 8, 2, 0},
		{&Sequence{Block: 0xffffffff, Count: 10}, 39, 9, 3},

		{&Sequence{Block: 0xffffffff, Count: 10, Next: &Sequence{Block: 0xcc000000, Count: 10}}, 79, 9, 3},
		{&Sequence{Block: 0xffffffff, Count: 10, Next: &Sequence{Block: 0xcc000000, Count: 10}}, 80, 0, -1},
	}

	for n, i := range input {
		_, _, precBlocks, inBlockBytePos := findSequence(i.head, i.bytePos)
		if precBlocks != i.precBlocks || inBlockBytePos != i.inBlockBytePos {
			t.Fatalf("Error in (%d) findSequence(). Expected (%d, %d). Got (%d, %d)", n, i.precBlocks, i.inBlockBytePos, precBlocks, inBlockBytePos)
		}
	}
}

func TestCheckIfAvailable(t *testing.T) {
	input := []struct {
		head    *Sequence
		ordinal int
		bytePos int
		bitPos  int
	}{
		{&Sequence{Block: 0xffffffff, Count: 0}, 0, -1, -1},
		{&Sequence{Block: 0xffffffff, Count: 0}, 31, -1, -1},
		{&Sequence{Block: 0xffffffff, Count: 0}, 100, -1, -1},

		{&Sequence{Block: 0x0, Count: 1}, 0, 0, 0},
		{&Sequence{Block: 0x0, Count: 1}, 1, 0, 1},
		{&Sequence{Block: 0x0, Count: 1}, 31, 3, 7},
		{&Sequence{Block: 0x0, Count: 1}, 60, -1, -1},

		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0x800000ff, Count: 1}}, 31, -1, -1},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0x800000ff, Count: 1}}, 32, -1, -1},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0x800000ff, Count: 1}}, 33, 4, 1},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xC00000ff, Count: 1}}, 33, -1, -1},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xC00000ff, Count: 1}}, 34, 4, 2},

		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xC00000ff, Count: 1, Next: &Sequence{Block: 0x0, Count: 1}}}, 55, 6, 7},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xC00000ff, Count: 1, Next: &Sequence{Block: 0x0, Count: 1}}}, 56, -1, -1},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xC00000ff, Count: 1, Next: &Sequence{Block: 0x0, Count: 1}}}, 63, -1, -1},

		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xC00000ff, Count: 1, Next: &Sequence{Block: 0x0, Count: 1}}}, 64, 8, 0},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xC00000ff, Count: 1, Next: &Sequence{Block: 0x0, Count: 1}}}, 95, 11, 7},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xC00000ff, Count: 1, Next: &Sequence{Block: 0x0, Count: 1}}}, 96, -1, -1},
	}

	for n, i := range input {
		bytePos, bitPos, _ := CheckIfAvailable(i.head, i.ordinal)
		if bytePos != i.bytePos || bitPos != i.bitPos {
			t.Fatalf("Error in (%d) checkIfAvailable(ord:%d). Expected (%d, %d). Got (%d, %d)", n, i.ordinal, i.bytePos, i.bitPos, bytePos, bitPos)
		}
	}
}

func TestMergeSequences(t *testing.T) {
	input := []struct {
		original *Sequence
		merged   *Sequence
	}{
		{&Sequence{Block: 0xFE000000, Count: 8, Next: &Sequence{Block: 0xFE000000, Count: 2}}, &Sequence{Block: 0xFE000000, Count: 10}},
		{&Sequence{Block: 0xFFFFFFFF, Count: 8, Next: &Sequence{Block: 0xFFFFFFFF, Count: 1}}, &Sequence{Block: 0xFFFFFFFF, Count: 9}},
		{&Sequence{Block: 0xFFFFFFFF, Count: 1, Next: &Sequence{Block: 0xFFFFFFFF, Count: 8}}, &Sequence{Block: 0xFFFFFFFF, Count: 9}},

		{&Sequence{Block: 0xFFFFFFF0, Count: 8, Next: &Sequence{Block: 0xFFFFFFF0, Count: 1}}, &Sequence{Block: 0xFFFFFFF0, Count: 9}},
		{&Sequence{Block: 0xFFFFFFF0, Count: 1, Next: &Sequence{Block: 0xFFFFFFF0, Count: 8}}, &Sequence{Block: 0xFFFFFFF0, Count: 9}},

		{&Sequence{Block: 0xFE, Count: 8, Next: &Sequence{Block: 0xFE, Count: 1, Next: &Sequence{Block: 0xFE, Count: 5}}}, &Sequence{Block: 0xFE, Count: 14}},
		{&Sequence{Block: 0xFE, Count: 8, Next: &Sequence{Block: 0xFE, Count: 1, Next: &Sequence{Block: 0xFE, Count: 5, Next: &Sequence{Block: 0xFF, Count: 1}}}},
			&Sequence{Block: 0xFE, Count: 14, Next: &Sequence{Block: 0xFF, Count: 1}}},

		// No merge
		{&Sequence{Block: 0xFE, Count: 8, Next: &Sequence{Block: 0xF8, Count: 1, Next: &Sequence{Block: 0xFE, Count: 5}}},
			&Sequence{Block: 0xFE, Count: 8, Next: &Sequence{Block: 0xF8, Count: 1, Next: &Sequence{Block: 0xFE, Count: 5}}}},

		// No merge from head: // Merge function tries to merge from passed head. If it can't merge with Next, it does not reattempt with Next as head
		{&Sequence{Block: 0xFE, Count: 8, Next: &Sequence{Block: 0xFF, Count: 1, Next: &Sequence{Block: 0xFF, Count: 5}}},
			&Sequence{Block: 0xFE, Count: 8, Next: &Sequence{Block: 0xFF, Count: 6}}},
	}

	for n, i := range input {
		mergeSequences(i.original)
		for !i.merged.Equal(i.original) {
			t.Fatalf("Error in (%d) mergeSequences().\nExp: %s\nGot: %s,", n, i.merged, i.original)
		}
	}
}

func TestPushReservation(t *testing.T) {
	input := []struct {
		mask    *Sequence
		bytePos int
		bitPos  int
		newMask *Sequence
	}{
		// Create first Sequence and fill in 8 addresses starting from address 0
		{&Sequence{Block: 0x0, Count: 8, Next: nil}, 0, 0, &Sequence{Block: 0x80000000, Count: 1, Next: &Sequence{Block: 0x0, Count: 7, Next: nil}}},
		{&Sequence{Block: 0x80000000, Count: 8}, 0, 1, &Sequence{Block: 0xC0000000, Count: 1, Next: &Sequence{Block: 0x80000000, Count: 7, Next: nil}}},
		{&Sequence{Block: 0xC0000000, Count: 8}, 0, 2, &Sequence{Block: 0xE0000000, Count: 1, Next: &Sequence{Block: 0xC0000000, Count: 7, Next: nil}}},
		{&Sequence{Block: 0xE0000000, Count: 8}, 0, 3, &Sequence{Block: 0xF0000000, Count: 1, Next: &Sequence{Block: 0xE0000000, Count: 7, Next: nil}}},
		{&Sequence{Block: 0xF0000000, Count: 8}, 0, 4, &Sequence{Block: 0xF8000000, Count: 1, Next: &Sequence{Block: 0xF0000000, Count: 7, Next: nil}}},
		{&Sequence{Block: 0xF8000000, Count: 8}, 0, 5, &Sequence{Block: 0xFC000000, Count: 1, Next: &Sequence{Block: 0xF8000000, Count: 7, Next: nil}}},
		{&Sequence{Block: 0xFC000000, Count: 8}, 0, 6, &Sequence{Block: 0xFE000000, Count: 1, Next: &Sequence{Block: 0xFC000000, Count: 7, Next: nil}}},
		{&Sequence{Block: 0xFE000000, Count: 8}, 0, 7, &Sequence{Block: 0xFF000000, Count: 1, Next: &Sequence{Block: 0xFE000000, Count: 7, Next: nil}}},

		{&Sequence{Block: 0x80000000, Count: 1, Next: &Sequence{Block: 0x0, Count: 7}}, 0, 1, &Sequence{Block: 0xC0000000, Count: 1, Next: &Sequence{Block: 0x0, Count: 7, Next: nil}}},

		// Create second Sequence and fill in 8 addresses starting from address 32
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0x00000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6, Next: nil}}}, 4, 0,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0x80000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0x80000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 1,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xC0000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xC0000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 2,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xE0000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xE0000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 3,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xF0000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xF0000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 4,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xF8000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xF8000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 5,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFC000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFC000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 6,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFE000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFE000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 4, 7,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFF000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		// fill in 8 addresses starting from address 40
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFF000000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 0,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFF800000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFF800000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 1,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFC00000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFC00000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 2,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFE00000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFE00000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 3,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFF00000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFF00000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 4,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFF80000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFF80000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 5,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFFC0000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFFC0000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 6,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFFE0000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFFE0000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}, 5, 7,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xFFFF0000, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 6}}}},

		// Insert new Sequence
		{&Sequence{Block: 0xffffffff, Count: 2, Next: &Sequence{Block: 0x0, Count: 6}}, 8, 0,
			&Sequence{Block: 0xffffffff, Count: 2, Next: &Sequence{Block: 0x80000000, Count: 1, Next: &Sequence{Block: 0x0, Count: 5}}}},
		{&Sequence{Block: 0xffffffff, Count: 2, Next: &Sequence{Block: 0x80000000, Count: 1, Next: &Sequence{Block: 0x0, Count: 5}}}, 8, 1,
			&Sequence{Block: 0xffffffff, Count: 2, Next: &Sequence{Block: 0xC0000000, Count: 1, Next: &Sequence{Block: 0x0, Count: 5}}}},

		// Merge affected with Next
		{&Sequence{Block: 0xffffffff, Count: 7, Next: &Sequence{Block: 0xfffffffe, Count: 2, Next: &Sequence{Block: 0xffffffff, Count: 1}}}, 31, 7,
			&Sequence{Block: 0xffffffff, Count: 8, Next: &Sequence{Block: 0xfffffffe, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 1}}}},
		{&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xfffffffc, Count: 1, Next: &Sequence{Block: 0xfffffffe, Count: 6}}}, 7, 6,
			&Sequence{Block: 0xffffffff, Count: 1, Next: &Sequence{Block: 0xfffffffe, Count: 7}}},

		// Merge affected with Next and Next.Next
		{&Sequence{Block: 0xffffffff, Count: 7, Next: &Sequence{Block: 0xfffffffe, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 1}}}, 31, 7,
			&Sequence{Block: 0xffffffff, Count: 9}},
		{&Sequence{Block: 0xffffffff, Count: 7, Next: &Sequence{Block: 0xfffffffe, Count: 1}}, 31, 7,
			&Sequence{Block: 0xffffffff, Count: 8}},

		// Merge affected with previous and Next
		{&Sequence{Block: 0xffffffff, Count: 7, Next: &Sequence{Block: 0xfffffffe, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 1}}}, 31, 7,
			&Sequence{Block: 0xffffffff, Count: 9}},

		// Redundant push: No change
		{&Sequence{Block: 0xffff0000, Count: 1}, 0, 0, &Sequence{Block: 0xffff0000, Count: 1}},
		{&Sequence{Block: 0xffff0000, Count: 7}, 25, 7, &Sequence{Block: 0xffff0000, Count: 7}},
		{&Sequence{Block: 0xffffffff, Count: 7, Next: &Sequence{Block: 0xfffffffe, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 1}}}, 7, 7,
			&Sequence{Block: 0xffffffff, Count: 7, Next: &Sequence{Block: 0xfffffffe, Count: 1, Next: &Sequence{Block: 0xffffffff, Count: 1}}}},
	}

	for n, i := range input {
		mask := PushReservation(i.bytePos, i.bitPos, i.mask, false)
		if !mask.Equal(i.newMask) {
			t.Fatalf("Error in (%d) pushReservation():\n%s + (%d,%d):\nExp: %s\nGot: %s,", n, i.mask, i.bytePos, i.bitPos, i.newMask, mask)
		}
	}
}

func TestSerializeDeserialize(t *testing.T) {
	s := &Sequence{
		Block: 0xffffffff,
		Count: 1,
		Next: &Sequence{
			Block: 0xFF000000,
			Count: 1,
			Next: &Sequence{
				Block: 0xffffffff,
				Count: 6,
				Next: &Sequence{
					Block: 0xffffffff,
					Count: 1,
					Next: &Sequence{
						Block: 0xFF800000,
						Count: 1,
						Next: &Sequence{
							Block: 0xffffffff,
							Count: 6,
						},
					},
				},
			},
		},
	}

	data, err := s.ToByteArray()
	if err != nil {
		t.Fatal(err)
	}

	r := &Sequence{}
	err = r.FromByteArray(data)
	if err != nil {
		t.Fatal(err)
	}

	if !s.Equal(r) {
		t.Fatalf("Sequences are different: \n%v\n%v", s, r)
	}
}
