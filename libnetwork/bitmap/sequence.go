// Package bitmap provides a datatype for long vectors of bits.
package bitmap

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

// block sequence constants
// If needed we can think of making these configurable
const (
	blockLen      = uint32(32)
	blockBytes    = uint64(blockLen / 8)
	blockMAX      = uint32(1<<blockLen - 1)
	blockFirstBit = uint32(1) << (blockLen - 1)
	invalidPos    = uint64(0xFFFFFFFFFFFFFFFF)
)

var (
	// ErrNoBitAvailable is returned when no more bits are available to set
	ErrNoBitAvailable = errors.New("no bit available")
	// ErrBitAllocated is returned when the specific bit requested is already set
	ErrBitAllocated = errors.New("requested bit is already allocated")
)

// https://github.com/golang/go/issues/8005#issuecomment-190753527
type noCopy struct{}

func (noCopy) Lock() {}

// Bitmap is a fixed-length bit vector. It is not safe for concurrent use.
//
// The data is stored as a list of run-length encoded blocks. It operates
// directly on the encoded representation, without decompressing.
type Bitmap struct {
	bits       uint64
	unselected uint64
	head       *sequence
	curr       uint64

	// Shallow copies would share the same head pointer but a copy of the
	// unselected count. Mutating the sequence through one would change the
	// bits for all copies but only update that one copy's unselected count,
	// which would result in subtle bugs.
	noCopy noCopy
}

// NewHandle returns a new Bitmap n bits long.
func New(n uint64) *Bitmap {
	return &Bitmap{
		bits:       n,
		unselected: n,
		head: &sequence{
			block: 0x0,
			count: getNumBlocks(n),
		},
	}
}

// Copy returns a deep copy of b.
func Copy(b *Bitmap) *Bitmap {
	return &Bitmap{
		bits:       b.bits,
		unselected: b.unselected,
		head:       b.head.getCopy(),
		curr:       b.curr,
	}
}

// sequence represents a recurring sequence of 32 bits long bitmasks
type sequence struct {
	block uint32    // block is a symbol representing 4 byte long allocation bitmask
	count uint64    // number of consecutive blocks (symbols)
	next  *sequence // next sequence
}

// String returns a string representation of the block sequence starting from this block
func (s *sequence) toString() string {
	var nextBlock string
	if s.next == nil {
		nextBlock = "end"
	} else {
		nextBlock = s.next.toString()
	}
	return fmt.Sprintf("(0x%x, %d)->%s", s.block, s.count, nextBlock)
}

// GetAvailableBit returns the position of the first unset bit in the bitmask represented by this sequence
func (s *sequence) getAvailableBit(from uint64) (uint64, uint64, error) {
	if s.block == blockMAX || s.count == 0 {
		return invalidPos, invalidPos, ErrNoBitAvailable
	}
	bits := from
	bitSel := blockFirstBit >> from
	for bitSel > 0 && s.block&bitSel != 0 {
		bitSel >>= 1
		bits++
	}
	// Check if the loop exited because it could not
	// find any available bit int block  starting from
	// "from". Return invalid pos in that case.
	if bitSel == 0 {
		return invalidPos, invalidPos, ErrNoBitAvailable
	}
	return bits / 8, bits % 8, nil
}

// GetCopy returns a copy of the linked list rooted at this node
func (s *sequence) getCopy() *sequence {
	n := &sequence{block: s.block, count: s.count}
	pn := n
	ps := s.next
	for ps != nil {
		pn.next = &sequence{block: ps.block, count: ps.count}
		pn = pn.next
		ps = ps.next
	}
	return n
}

// Equal checks if this sequence is equal to the passed one
func (s *sequence) equal(o *sequence) bool {
	this := s
	other := o
	for this != nil {
		if other == nil {
			return false
		}
		if this.block != other.block || this.count != other.count {
			return false
		}
		this = this.next
		other = other.next
	}
	return other == nil
}

// ToByteArray converts the sequence into a byte array
func (s *sequence) toByteArray() ([]byte, error) {
	var bb []byte

	p := s
	b := make([]byte, 12)
	for p != nil {
		binary.BigEndian.PutUint32(b[0:], p.block)
		binary.BigEndian.PutUint64(b[4:], p.count)
		bb = append(bb, b...)
		p = p.next
	}

	return bb, nil
}

// fromByteArray construct the sequence from the byte array
func (s *sequence) fromByteArray(data []byte) error {
	l := len(data)
	if l%12 != 0 {
		return fmt.Errorf("cannot deserialize byte sequence of length %d (%v)", l, data)
	}

	p := s
	i := 0
	for {
		p.block = binary.BigEndian.Uint32(data[i : i+4])
		p.count = binary.BigEndian.Uint64(data[i+4 : i+12])
		i += 12
		if i == l {
			break
		}
		p.next = &sequence{}
		p = p.next
	}

	return nil
}

// SetAnyInRange sets the first unset bit in the range [start, end) and returns
// the ordinal of the set bit.
//
// When serial=true, the bitmap is scanned starting from the ordinal following
// the bit most recently set by [Bitmap.SetAny] or [Bitmap.SetAnyInRange].
func (h *Bitmap) SetAnyInRange(start, end uint64, serial bool) (uint64, error) {
	if end < start || end >= h.bits {
		return invalidPos, fmt.Errorf("invalid bit range [%d, %d)", start, end)
	}
	if h.Unselected() == 0 {
		return invalidPos, ErrNoBitAvailable
	}
	return h.set(0, start, end, true, false, serial)
}

// SetAny sets the first unset bit in the sequence and returns the ordinal of
// the set bit.
//
// When serial=true, the bitmap is scanned starting from the ordinal following
// the bit most recently set by [Bitmap.SetAny] or [Bitmap.SetAnyInRange].
func (h *Bitmap) SetAny(serial bool) (uint64, error) {
	if h.Unselected() == 0 {
		return invalidPos, ErrNoBitAvailable
	}
	return h.set(0, 0, h.bits-1, true, false, serial)
}

// Set atomically sets the corresponding bit in the sequence
func (h *Bitmap) Set(ordinal uint64) error {
	if err := h.validateOrdinal(ordinal); err != nil {
		return err
	}
	_, err := h.set(ordinal, 0, 0, false, false, false)
	return err
}

// Unset atomically unsets the corresponding bit in the sequence
func (h *Bitmap) Unset(ordinal uint64) error {
	if err := h.validateOrdinal(ordinal); err != nil {
		return err
	}
	_, err := h.set(ordinal, 0, 0, false, true, false)
	return err
}

// IsSet atomically checks if the ordinal bit is set. In case ordinal
// is outside of the bit sequence limits, false is returned.
func (h *Bitmap) IsSet(ordinal uint64) bool {
	if err := h.validateOrdinal(ordinal); err != nil {
		return false
	}
	_, _, err := checkIfAvailable(h.head, ordinal)
	return err != nil
}

// CheckConsistency checks if the bit sequence is in an inconsistent state and attempts to fix it.
// It looks for a corruption signature that may happen in docker 1.9.0 and 1.9.1.
func (h *Bitmap) CheckConsistency() bool {
	corrupted := false
	for p, c := h.head, h.head.next; c != nil; c = c.next {
		if c.count == 0 {
			corrupted = true
			p.next = c.next
			continue // keep same p
		}
		p = c
	}
	return corrupted
}

// set/reset the bit
func (h *Bitmap) set(ordinal, start, end uint64, any bool, release bool, serial bool) (uint64, error) {
	var (
		bitPos  uint64
		bytePos uint64
		ret     uint64
		err     error
	)

	curr := uint64(0)
	if serial {
		curr = h.curr
	}
	// Get position if available
	if release {
		bytePos, bitPos = ordinalToPos(ordinal)
	} else {
		if any {
			bytePos, bitPos, err = getAvailableFromCurrent(h.head, start, curr, end)
			ret = posToOrdinal(bytePos, bitPos)
			if err == nil {
				h.curr = ret + 1
			}
		} else {
			bytePos, bitPos, err = checkIfAvailable(h.head, ordinal)
			ret = ordinal
		}
	}
	if err != nil {
		return ret, err
	}

	h.head = pushReservation(bytePos, bitPos, h.head, release)
	if release {
		h.unselected++
	} else {
		h.unselected--
	}

	return ret, nil
}

// checks is needed because to cover the case where the number of bits is not a multiple of blockLen
func (h *Bitmap) validateOrdinal(ordinal uint64) error {
	if ordinal >= h.bits {
		return errors.New("bit does not belong to the sequence")
	}
	return nil
}

// MarshalBinary encodes h into a binary representation.
func (h *Bitmap) MarshalBinary() ([]byte, error) {
	ba := make([]byte, 16)
	binary.BigEndian.PutUint64(ba[0:], h.bits)
	binary.BigEndian.PutUint64(ba[8:], h.unselected)
	bm, err := h.head.toByteArray()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize head: %v", err)
	}
	ba = append(ba, bm...)

	return ba, nil
}

// UnmarshalBinary decodes a binary representation of a Bitmap value which was
// generated using [Bitmap.MarshalBinary].
//
// The scan position for serial [Bitmap.SetAny] and [Bitmap.SetAnyInRange]
// operations is neither unmarshaled nor reset.
func (h *Bitmap) UnmarshalBinary(ba []byte) error {
	if ba == nil {
		return errors.New("nil byte array")
	}

	nh := &sequence{}
	err := nh.fromByteArray(ba[16:])
	if err != nil {
		return fmt.Errorf("failed to deserialize head: %v", err)
	}

	h.head = nh
	h.bits = binary.BigEndian.Uint64(ba[0:8])
	h.unselected = binary.BigEndian.Uint64(ba[8:16])
	return nil
}

// Bits returns the length of the bit sequence
func (h *Bitmap) Bits() uint64 {
	return h.bits
}

// Unselected returns the number of bits which are not selected
func (h *Bitmap) Unselected() uint64 {
	return h.unselected
}

func (h *Bitmap) String() string {
	return fmt.Sprintf("Bits: %d, Unselected: %d, Sequence: %s Curr:%d",
		h.bits, h.unselected, h.head.toString(), h.curr)
}

// MarshalJSON encodes h into a JSON message
func (h *Bitmap) MarshalJSON() ([]byte, error) {
	b, err := h.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return json.Marshal(b)
}

// UnmarshalJSON decodes JSON message into h
func (h *Bitmap) UnmarshalJSON(data []byte) error {
	var b []byte
	if err := json.Unmarshal(data, &b); err != nil {
		return err
	}
	return h.UnmarshalBinary(b)
}

// getFirstAvailable looks for the first unset bit in passed mask starting from start
func getFirstAvailable(head *sequence, start uint64) (uint64, uint64, error) {
	// Find sequence which contains the start bit
	byteStart, bitStart := ordinalToPos(start)
	current, _, precBlocks, inBlockBytePos := findSequence(head, byteStart)
	// Derive the this sequence offsets
	byteOffset := byteStart - inBlockBytePos
	bitOffset := inBlockBytePos*8 + bitStart
	for current != nil {
		if current.block != blockMAX {
			// If the current block is not full, check if there is any bit
			// from the current bit in the current block. If not, before proceeding to the
			// next block node, make sure we check for available bit in the next
			// instance of the same block. Due to RLE same block signature will be
			// compressed.
		retry:
			bytePos, bitPos, err := current.getAvailableBit(bitOffset)
			if err != nil && precBlocks == current.count-1 {
				// This is the last instance in the same block node,
				// so move to the next block.
				goto next
			}
			if err != nil {
				// There are some more instances of the same block, so add the offset
				// and be optimistic that you will find the available bit in the next
				// instance of the same block.
				bitOffset = 0
				byteOffset += blockBytes
				precBlocks++
				goto retry
			}
			return byteOffset + bytePos, bitPos, err
		}
		// Moving to next block: Reset bit offset.
	next:
		bitOffset = 0
		byteOffset += (current.count * blockBytes) - (precBlocks * blockBytes)
		precBlocks = 0
		current = current.next
	}
	return invalidPos, invalidPos, ErrNoBitAvailable
}

// getAvailableFromCurrent will look for available ordinal from the current ordinal.
// If none found then it will loop back to the start to check of the available bit.
// This can be further optimized to check from start till curr in case of a rollover
func getAvailableFromCurrent(head *sequence, start, curr, end uint64) (uint64, uint64, error) {
	var bytePos, bitPos uint64
	var err error
	if curr != 0 && curr > start {
		bytePos, bitPos, err = getFirstAvailable(head, curr)
		ret := posToOrdinal(bytePos, bitPos)
		if end < ret || err != nil {
			goto begin
		}
		return bytePos, bitPos, nil
	}

begin:
	bytePos, bitPos, err = getFirstAvailable(head, start)
	ret := posToOrdinal(bytePos, bitPos)
	if end < ret || err != nil {
		return invalidPos, invalidPos, ErrNoBitAvailable
	}
	return bytePos, bitPos, nil
}

// checkIfAvailable checks if the bit correspondent to the specified ordinal is unset
// If the ordinal is beyond the sequence limits, a negative response is returned
func checkIfAvailable(head *sequence, ordinal uint64) (uint64, uint64, error) {
	bytePos, bitPos := ordinalToPos(ordinal)

	// Find the sequence containing this byte
	current, _, _, inBlockBytePos := findSequence(head, bytePos)
	if current != nil {
		// Check whether the bit corresponding to the ordinal address is unset
		bitSel := blockFirstBit >> (inBlockBytePos*8 + bitPos)
		if current.block&bitSel == 0 {
			return bytePos, bitPos, nil
		}
	}

	return invalidPos, invalidPos, ErrBitAllocated
}

// Given the byte position and the sequences list head, return the pointer to the
// sequence containing the byte (current), the pointer to the previous sequence,
// the number of blocks preceding the block containing the byte inside the current sequence.
// If bytePos is outside of the list, function will return (nil, nil, 0, invalidPos)
func findSequence(head *sequence, bytePos uint64) (*sequence, *sequence, uint64, uint64) {
	// Find the sequence containing this byte
	previous := head
	current := head
	n := bytePos
	for current.next != nil && n >= (current.count*blockBytes) { // Nil check for less than 32 addresses masks
		n -= (current.count * blockBytes)
		previous = current
		current = current.next
	}

	// If byte is outside of the list, let caller know
	if n >= (current.count * blockBytes) {
		return nil, nil, 0, invalidPos
	}

	// Find the byte position inside the block and the number of blocks
	// preceding the block containing the byte inside this sequence
	precBlocks := n / blockBytes
	inBlockBytePos := bytePos % blockBytes

	return current, previous, precBlocks, inBlockBytePos
}

// PushReservation pushes the bit reservation inside the bitmask.
// Given byte and bit positions, identify the sequence (current) which holds the block containing the affected bit.
// Create a new block with the modified bit according to the operation (allocate/release).
// Create a new sequence containing the new block and insert it in the proper position.
// Remove current sequence if empty.
// Check if new sequence can be merged with neighbour (previous/next) sequences.
//
// Identify "current" sequence containing block:
//
//	[prev seq] [current seq] [next seq]
//
// Based on block position, resulting list of sequences can be any of three forms:
//
// block position                        Resulting list of sequences
//
// A) block is first in current:         [prev seq] [new] [modified current seq] [next seq]
// B) block is last in current:          [prev seq] [modified current seq] [new] [next seq]
// C) block is in the middle of current: [prev seq] [curr pre] [new] [curr post] [next seq]
func pushReservation(bytePos, bitPos uint64, head *sequence, release bool) *sequence {
	// Store list's head
	newHead := head

	// Find the sequence containing this byte
	current, previous, precBlocks, inBlockBytePos := findSequence(head, bytePos)
	if current == nil {
		return newHead
	}

	// Construct updated block
	bitSel := blockFirstBit >> (inBlockBytePos*8 + bitPos)
	newBlock := current.block
	if release {
		newBlock &^= bitSel
	} else {
		newBlock |= bitSel
	}

	// Quit if it was a redundant request
	if current.block == newBlock {
		return newHead
	}

	// Current sequence inevitably looses one block, upadate count
	current.count--

	// Create new sequence
	newSequence := &sequence{block: newBlock, count: 1}

	// Insert the new sequence in the list based on block position
	if precBlocks == 0 { // First in sequence (A)
		newSequence.next = current
		if current == head {
			newHead = newSequence
			previous = newHead
		} else {
			previous.next = newSequence
		}
		removeCurrentIfEmpty(&newHead, newSequence, current)
		mergeSequences(previous)
	} else if precBlocks == current.count { // Last in sequence (B)
		newSequence.next = current.next
		current.next = newSequence
		mergeSequences(current)
	} else { // In between the sequence (C)
		currPre := &sequence{block: current.block, count: precBlocks, next: newSequence}
		currPost := current
		currPost.count -= precBlocks
		newSequence.next = currPost
		if currPost == head {
			newHead = currPre
		} else {
			previous.next = currPre
		}
		// No merging or empty current possible here
	}

	return newHead
}

// Removes the current sequence from the list if empty, adjusting the head pointer if needed
func removeCurrentIfEmpty(head **sequence, previous, current *sequence) {
	if current.count == 0 {
		if current == *head {
			*head = current.next
		} else {
			previous.next = current.next
		}
	}
}

// Given a pointer to a sequence, it checks if it can be merged with any following sequences
// It stops when no more merging is possible.
// TODO: Optimization: only attempt merge from start to end sequence, no need to scan till the end of the list
func mergeSequences(seq *sequence) {
	if seq != nil {
		// Merge all what possible from seq
		for seq.next != nil && seq.block == seq.next.block {
			seq.count += seq.next.count
			seq.next = seq.next.next
		}
		// Move to next
		mergeSequences(seq.next)
	}
}

func getNumBlocks(numBits uint64) uint64 {
	numBlocks := numBits / uint64(blockLen)
	if numBits%uint64(blockLen) != 0 {
		numBlocks++
	}
	return numBlocks
}

func ordinalToPos(ordinal uint64) (uint64, uint64) {
	return ordinal / 8, ordinal % 8
}

func posToOrdinal(bytePos, bitPos uint64) uint64 {
	return bytePos*8 + bitPos
}
