// Package bitseq provides a structure and utilities for representing long bitmask
// as sequence of run-lenght encoded blocks. It operates direclty on the encoded
// representation, it does not decode/encode.
package bitseq

import (
	"fmt"

	"github.com/docker/libnetwork/netutils"
)

// Block Sequence constants
// If needed we can think of making these configurable
const (
	blockLen      = 32
	blockBytes    = blockLen / 8
	blockMAX      = 1<<blockLen - 1
	blockFirstBit = 1 << (blockLen - 1)
)

// Handle contains the sequece representing the bitmask and its identifier
type Handle struct {
	ID   string
	Head *Sequence
}

// NewHandle returns an instance of the bitmask handler
func NewHandle(id string, numElements uint32) *Handle {
	return &Handle{
		ID: id,
		Head: &Sequence{
			Block: 0x0,
			Count: getNumBlocks(numElements),
			Next:  nil,
		},
	}
}

// Sequence reresents a recurring sequence of 32 bits long bitmasks
type Sequence struct {
	Block uint32    // block representing 4 byte long allocation bitmask
	Count uint32    // number of consecutive blocks
	Next  *Sequence // next sequence
}

// NewSequence returns a sequence initialized to represent a bitmaks of numElements bits
func NewSequence(numElements uint32) *Sequence {
	return &Sequence{Block: 0x0, Count: getNumBlocks(numElements), Next: nil}
}

// String returns a string representation of the block sequence starting from this block
func (s *Sequence) String() string {
	var nextBlock string
	if s.Next == nil {
		nextBlock = "end"
	} else {
		nextBlock = s.Next.String()
	}
	return fmt.Sprintf("(0x%x, %d)->%s", s.Block, s.Count, nextBlock)
}

// GetAvailableBit returns the position of the first unset bit in the bitmask represented by this sequence
func (s *Sequence) GetAvailableBit() (bytePos, bitPos int) {
	if s.Block == blockMAX || s.Count == 0 {
		return -1, -1
	}
	bits := 0
	bitSel := uint32(blockFirstBit)
	for bitSel > 0 && s.Block&bitSel != 0 {
		bitSel >>= 1
		bits++
	}
	return bits / 8, bits % 8
}

// Equal checks if this sequence is equal to the passed one
func (s *Sequence) Equal(o *Sequence) bool {
	this := s
	other := o
	for this != nil {
		if other == nil {
			return false
		}
		if this.Block != other.Block || this.Count != other.Count {
			return false
		}
		this = this.Next
		other = other.Next
	}
	// Check if other is longer than this
	if other != nil {
		return false
	}
	return true
}

// ToByteArray converts the sequence into a byte array
// TODO (aboch): manage network/host order stuff
func (s *Sequence) ToByteArray() ([]byte, error) {
	var bb []byte

	p := s
	for p != nil {
		bb = append(bb, netutils.U32ToA(p.Block)...)
		bb = append(bb, netutils.U32ToA(p.Count)...)
		p = p.Next
	}

	return bb, nil
}

// FromByteArray construct the sequence from the byte array
// TODO (aboch): manage network/host order stuff
func (s *Sequence) FromByteArray(data []byte) error {
	l := len(data)
	if l%8 != 0 {
		return fmt.Errorf("cannot deserialize byte sequence of lenght %d", l)
	}

	p := s
	i := 0
	for {
		p.Block = netutils.ATo32(data[i : i+4])
		p.Count = netutils.ATo32(data[i+4 : i+8])
		i += 8
		if i == l {
			break
		}
		p.Next = &Sequence{}
		p = p.Next
	}

	return nil
}

// GetFirstAvailable returns the byte and bit position of the first unset bit
func (h *Handle) GetFirstAvailable() (int, int) {
	return GetFirstAvailable(h.Head)
}

// CheckIfAvailable checks if the bit correspondent to the specified ordinal is unset
// If the ordinal is beyond the Sequence limits, a negative response is returned
func (h *Handle) CheckIfAvailable(ordinal int) (int, int) {
	return CheckIfAvailable(h.Head, ordinal)
}

// PushReservation pushes the bit reservation inside the bitmask.
func (h *Handle) PushReservation(bytePos, bitPos int, release bool) {
	h.Head = PushReservation(bytePos, bitPos, h.Head, release)
}

// GetFirstAvailable looks for the first unset bit in passed mask
func GetFirstAvailable(head *Sequence) (int, int) {
	byteIndex := 0
	current := head
	for current != nil {
		if current.Block != blockMAX {
			bytePos, bitPos := current.GetAvailableBit()
			return byteIndex + bytePos, bitPos
		}
		byteIndex += int(current.Count * blockBytes)
		current = current.Next
	}
	return -1, -1
}

// CheckIfAvailable checks if the bit correspondent to the specified ordinal is unset
// If the ordinal is beyond the Sequence limits, a negative response is returned
func CheckIfAvailable(head *Sequence, ordinal int) (int, int) {
	bytePos := ordinal / 8
	bitPos := ordinal % 8

	// Find the Sequence containing this byte
	current, _, _, inBlockBytePos := findSequence(head, bytePos)

	if current != nil {
		// Check whether the bit corresponding to the ordinal address is unset
		bitSel := uint32(blockFirstBit >> uint(inBlockBytePos*8+bitPos))
		if current.Block&bitSel == 0 {
			return bytePos, bitPos
		}
	}

	return -1, -1
}

// Given the byte position and the sequences list head, return the pointer to the
// sequence containing the byte (current), the pointer to the previous sequence,
// the number of blocks preceding the block containing the byte inside the current sequence.
// If bytePos is outside of the list, function will return (nil, nil, 0, -1)
func findSequence(head *Sequence, bytePos int) (*Sequence, *Sequence, uint32, int) {
	// Find the Sequence containing this byte
	previous := head
	current := head
	n := bytePos
	for current.Next != nil && n >= int(current.Count*blockBytes) { // Nil check for less than 32 addresses masks
		n -= int(current.Count * blockBytes)
		previous = current
		current = current.Next
	}

	// If byte is outside of the list, let caller know
	if n >= int(current.Count*blockBytes) {
		return nil, nil, 0, -1
	}

	// Find the byte position inside the block and the number of blocks
	// preceding the block containing the byte inside this sequence
	precBlocks := uint32(n / blockBytes)
	inBlockBytePos := bytePos % blockBytes

	return current, previous, precBlocks, inBlockBytePos
}

// PushReservation pushes the bit reservation inside the bitmask.
// Given byte and bit positions, identify the sequence (current) which holds the block containing the affected bit.
// Create a new block with the modified bit according to the operation (allocate/release).
// Create a new Sequence containing the new Block and insert it in the proper position.
// Remove current sequence if empty.
// Check if new Sequence can be merged with neighbour (previous/Next) sequences.
//
//
// Identify "current" Sequence containing block:
//                                      [prev seq] [current seq] [Next seq]
//
// Based on block position, resulting list of sequences can be any of three forms:
//
//        Block position                        Resulting list of sequences
// A) Block is first in current:         [prev seq] [new] [modified current seq] [Next seq]
// B) Block is last in current:          [prev seq] [modified current seq] [new] [Next seq]
// C) Block is in the middle of current: [prev seq] [curr pre] [new] [curr post] [Next seq]
func PushReservation(bytePos, bitPos int, head *Sequence, release bool) *Sequence {
	// Store list's head
	newHead := head

	// Find the Sequence containing this byte
	current, previous, precBlocks, inBlockBytePos := findSequence(head, bytePos)
	if current == nil {
		return newHead
	}

	// Construct updated block
	bitSel := uint32(blockFirstBit >> uint(inBlockBytePos*8+bitPos))
	newBlock := current.Block
	if release {
		newBlock &^= bitSel
	} else {
		newBlock |= bitSel
	}

	// Quit if it was a redundant request
	if current.Block == newBlock {
		return newHead
	}

	// Current Sequence inevitably looses one block, upadate Count
	current.Count--

	// Create new sequence
	newSequence := &Sequence{Block: newBlock, Count: 1}

	// Insert the new sequence in the list based on block position
	if precBlocks == 0 { // First in sequence (A)
		newSequence.Next = current
		if current == head {
			newHead = newSequence
			previous = newHead
		} else {
			previous.Next = newSequence
		}
		removeCurrentIfEmpty(&newHead, newSequence, current)
		mergeSequences(previous)
	} else if precBlocks == current.Count-2 { // Last in sequence (B)
		newSequence.Next = current.Next
		current.Next = newSequence
		mergeSequences(current)
	} else { // In between the sequence (C)
		currPre := &Sequence{Block: current.Block, Count: precBlocks, Next: newSequence}
		currPost := current
		currPost.Count -= precBlocks
		newSequence.Next = currPost
		if currPost == head {
			newHead = currPre
		} else {
			previous.Next = currPre
		}
		// No merging or empty current possible here
	}

	return newHead
}

// Removes the current sequence from the list if empty, adjusting the head pointer if needed
func removeCurrentIfEmpty(head **Sequence, previous, current *Sequence) {
	if current.Count == 0 {
		if current == *head {
			*head = current.Next
		} else {
			previous.Next = current.Next
			current = current.Next
		}
	}
}

// Given a pointer to a Sequence, it checks if it can be merged with any following sequences
// It stops when no more merging is possible.
// TODO: Optimization: only attempt merge from start to end sequence, no need to scan till the end of the list
func mergeSequences(seq *Sequence) {
	if seq != nil {
		// Merge all what possible from seq
		for seq.Next != nil && seq.Block == seq.Next.Block {
			seq.Count += seq.Next.Count
			seq.Next = seq.Next.Next
		}
		// Move to Next
		mergeSequences(seq.Next)
	}
}

// Serialize converts the sequence into a byte array
func Serialize(head *Sequence) ([]byte, error) {
	return nil, nil
}

// Deserialize decodes the byte array into a sequence
func Deserialize(data []byte) (*Sequence, error) {
	return nil, nil
}

func getNumBlocks(numBits uint32) uint32 {
	numBlocks := numBits / blockLen
	if numBits%blockLen != 0 {
		numBlocks++
	}
	return numBlocks
}
