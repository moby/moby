// Package bitseq provides a structure and utilities for representing long bitmask
// as sequence of run-lenght encoded blocks. It operates direclty on the encoded
// representation, it does not decode/encode.
package bitseq

import (
	"fmt"
	"sync"

	"github.com/docker/libnetwork/datastore"
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
	bits       uint32
	unselected uint32
	head       *Sequence
	app        string
	id         string
	dbIndex    uint64
	store      datastore.DataStore
	sync.Mutex
}

// NewHandle returns a thread-safe instance of the bitmask handler
func NewHandle(app string, ds datastore.DataStore, id string, numElements uint32) (*Handle, error) {
	h := &Handle{
		app:        app,
		id:         id,
		store:      ds,
		bits:       numElements,
		unselected: numElements,
		head: &Sequence{
			Block: 0x0,
			Count: getNumBlocks(numElements),
		},
	}

	if h.store == nil {
		return h, nil
	}

	// Register for status changes
	h.watchForChanges()

	// Get the initial status from the ds if present.
	// We will be getting an instance without a dbIndex
	// (GetObject() does not set it): It is ok for now,
	// it will only cause the first allocation on this
	// node to go through a retry.
	var bah []byte
	if err := h.store.GetObject(datastore.Key(h.Key()...), &bah); err != nil {
		if err != datastore.ErrKeyNotFound {
			return nil, err
		}
		return h, nil
	}
	err := h.FromByteArray(bah)

	return h, err
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

// GetCopy returns a copy of the linked list rooted at this node
func (s *Sequence) GetCopy() *Sequence {
	n := &Sequence{Block: s.Block, Count: s.Count}
	pn := n
	ps := s.Next
	for ps != nil {
		pn.Next = &Sequence{Block: ps.Block, Count: ps.Count}
		pn = pn.Next
		ps = ps.Next
	}
	return n
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
		return fmt.Errorf("cannot deserialize byte sequence of lenght %d (%v)", l, data)
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
func (h *Handle) GetFirstAvailable() (int, int, error) {
	h.Lock()
	defer h.Unlock()
	return GetFirstAvailable(h.head)
}

// CheckIfAvailable checks if the bit correspondent to the specified ordinal is unset
// If the ordinal is beyond the Sequence limits, a negative response is returned
func (h *Handle) CheckIfAvailable(ordinal int) (int, int, error) {
	h.Lock()
	defer h.Unlock()
	return CheckIfAvailable(h.head, ordinal)
}

// PushReservation pushes the bit reservation inside the bitmask.
func (h *Handle) PushReservation(bytePos, bitPos int, release bool) error {
	// Create a copy of the current handler
	h.Lock()
	nh := &Handle{app: h.app, id: h.id, store: h.store, dbIndex: h.dbIndex, head: h.head.GetCopy()}
	h.Unlock()

	nh.head = PushReservation(bytePos, bitPos, nh.head, release)

	err := nh.writeToStore()
	if err == nil {
		// Commit went through, save locally
		h.Lock()
		h.head = nh.head
		if release {
			h.unselected++
		} else {
			h.unselected--
		}
		h.dbIndex = nh.dbIndex
		h.Unlock()
	}

	return err
}

// Destroy removes from the datastore the data belonging to this handle
func (h *Handle) Destroy() {
	h.deleteFromStore()
}

// ToByteArray converts this handle's data into a byte array
func (h *Handle) ToByteArray() ([]byte, error) {
	ba := make([]byte, 8)

	h.Lock()
	defer h.Unlock()
	copy(ba[0:4], netutils.U32ToA(h.bits))
	copy(ba[4:8], netutils.U32ToA(h.unselected))
	bm, err := h.head.ToByteArray()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize head: %s", err.Error())
	}
	ba = append(ba, bm...)

	return ba, nil
}

// FromByteArray reads his handle's data from a byte array
func (h *Handle) FromByteArray(ba []byte) error {
	if ba == nil {
		return fmt.Errorf("nil byte array")
	}

	nh := &Sequence{}
	err := nh.FromByteArray(ba[8:])
	if err != nil {
		return fmt.Errorf("failed to deserialize head: %s", err.Error())
	}

	h.Lock()
	h.head = nh
	h.bits = netutils.ATo32(ba[0:4])
	h.unselected = netutils.ATo32(ba[4:8])
	h.Unlock()

	return nil
}

// Bits returns the length of the bit sequence
func (h *Handle) Bits() uint32 {
	return h.bits
}

// Unselected returns the number of bits which are not selected
func (h *Handle) Unselected() uint32 {
	h.Lock()
	defer h.Unlock()
	return h.unselected
}

func (h *Handle) getDBIndex() uint64 {
	h.Lock()
	defer h.Unlock()
	return h.dbIndex
}

// GetFirstAvailable looks for the first unset bit in passed mask
func GetFirstAvailable(head *Sequence) (int, int, error) {
	byteIndex := 0
	current := head
	for current != nil {
		if current.Block != blockMAX {
			bytePos, bitPos := current.GetAvailableBit()
			return byteIndex + bytePos, bitPos, nil
		}
		byteIndex += int(current.Count * blockBytes)
		current = current.Next
	}
	return -1, -1, fmt.Errorf("no bit available")
}

// CheckIfAvailable checks if the bit correspondent to the specified ordinal is unset
// If the ordinal is beyond the Sequence limits, a negative response is returned
func CheckIfAvailable(head *Sequence, ordinal int) (int, int, error) {
	bytePos := ordinal / 8
	bitPos := ordinal % 8

	// Find the Sequence containing this byte
	current, _, _, inBlockBytePos := findSequence(head, bytePos)

	if current != nil {
		// Check whether the bit corresponding to the ordinal address is unset
		bitSel := uint32(blockFirstBit >> uint(inBlockBytePos*8+bitPos))
		if current.Block&bitSel == 0 {
			return bytePos, bitPos, nil
		}
	}

	return -1, -1, fmt.Errorf("requested bit is not available")
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

func getNumBlocks(numBits uint32) uint32 {
	numBlocks := numBits / blockLen
	if numBits%blockLen != 0 {
		numBlocks++
	}
	return numBlocks
}
