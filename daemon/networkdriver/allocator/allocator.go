package allocator

import (
	"errors"
	"math/big"
)

type slotStatus uint

var (
	bigOne = big.NewInt(1)

	slotAvailable slotStatus = 0
	slotAllocated slotStatus = 1

	ErrAlreadyAllocated = errors.New("requested slot is already allocated")
	ErrRangeFull        = errors.New("no more available slots in range")
)

type pool interface {
	GetSlotStatus(*big.Int) slotStatus
	SetSlotStatus(*big.Int, slotStatus)
}

// Indices in the automaticPool are rebased to 0 to keep the `big.Int` compact.
type automaticPool struct {
	bitfield   *big.Int
	last       *big.Int
	rangeBegin *big.Int
	rangeEnd   *big.Int
	rangeSize  *big.Int
}

func newAutomaticPool(rangeBegin, rangeEnd *big.Int) *automaticPool {
	rangeSize := big.NewInt(0).Sub(rangeEnd, rangeBegin)
	return &automaticPool{
		bitfield:   big.NewInt(0),
		last:       big.NewInt(0).Sub(rangeBegin, bigOne),
		rangeBegin: big.NewInt(0).Set(rangeBegin),
		rangeEnd:   big.NewInt(0).Set(rangeEnd),
		rangeSize:  rangeSize.Add(rangeSize, bigOne), // range is inclusive
	}
}

func (pool *automaticPool) GetSlotStatus(idx *big.Int) slotStatus {
	rebasedIdx := big.NewInt(0).Sub(idx, pool.rangeBegin)
	return slotStatus(pool.bitfield.Bit(int(rebasedIdx.Int64())))
}

func (pool *automaticPool) SetSlotStatus(idx *big.Int, status slotStatus) {
	if pool.IsInRange(idx) {
		rebasedIdx := big.NewInt(0).Sub(idx, pool.rangeBegin).Int64()
		pool.bitfield.SetBit(pool.bitfield, int(rebasedIdx), uint(status))
	}
}

func (pool *automaticPool) Allocate() (*big.Int, error) {
	candidate := big.NewInt(0).Add(pool.last, bigOne)
	for i := big.NewInt(0); i.Cmp(pool.rangeSize) < 0; i.Add(i, bigOne) {
		// If we go past the end of the range, loop back to the beginning to
		// check if previously allocated slots have been released in the
		// [begin, last[ range.
		if candidate.Cmp(pool.rangeEnd) > 0 {
			candidate.Set(pool.rangeBegin)
		}

		if pool.GetSlotStatus(candidate) == slotAvailable {
			pool.SetSlotStatus(candidate, slotAllocated)
			pool.last.Set(candidate)
			return candidate, nil
		}

		candidate.Add(candidate, bigOne)
	}
	return nil, ErrRangeFull
}

func (pool *automaticPool) IsInRange(idx *big.Int) bool {
	return idx.Cmp(pool.rangeBegin) >= 0 && idx.Cmp(pool.rangeEnd) <= 0
}

func (pool *automaticPool) ReleaseAll() {
	pool.bitfield.Set(big.NewInt(0))
}

func (pool *automaticPool) Size() *big.Int {
	return pool.rangeSize
}

// The custom pool simply holds a string representation of each allocated slot.
type customPool map[string]struct{}

func newCustomPool() customPool {
	return make(map[string]struct{})
}

func (pool customPool) GetSlotStatus(idx *big.Int) slotStatus {
	if _, ok := pool[idx.String()]; ok {
		return slotAllocated
	}
	return slotAvailable
}

func (pool customPool) SetSlotStatus(idx *big.Int, status slotStatus) {
	if status == slotAvailable {
		delete(pool, idx.String())
	} else {
		pool[idx.String()] = struct{}{}
	}
}

func (pool customPool) ReleaseAll() {
	pool = make(map[string]struct{})
}

// Allocator is a thread-unsafe implementation of an allocation strategy which
// offers both:
//	- automatic sequential allocation inside a specified range
//	- custom allocation of discrete slots.
// It is left untyped in order to be reused for ports, IP address, or any need
// other dynamic allocation inside a finite set.
type Allocator struct {
	customPool customPool
	autoPool   *automaticPool
}

// Create a new allocator providing the inclusive boundaries of the automatic
// allocation pool.
func NewAllocator(automaticRangeBegin, automaticRangeEnd *big.Int) *Allocator {
	return &Allocator{
		autoPool:   newAutomaticPool(automaticRangeBegin, automaticRangeEnd),
		customPool: newCustomPool(),
	}
}

// Attempt to allocate the specified index in the range.
func (allocator *Allocator) Allocate(candidate *big.Int) (err error) {
	var targetPool pool = allocator.selectPool(candidate)
	if targetPool.GetSlotStatus(candidate) == slotAllocated {
		err = ErrAlreadyAllocated
	}
	targetPool.SetSlotStatus(candidate, slotAllocated)
	return
}

// Attempt to find an available slot in the automatic pool.
func (allocator *Allocator) AllocateFirstAvailable() (*big.Int, error) {
	return allocator.autoPool.Allocate()
}

func (allocator *Allocator) AutomaticPoolSize() *big.Int {
	return allocator.autoPool.Size()
}

func (allocator *Allocator) Release(idx *big.Int) {
	allocator.selectPool(idx).SetSlotStatus(idx, slotAvailable)
}

func (allocator *Allocator) ReleaseAll() {
	allocator.autoPool.ReleaseAll()
	allocator.customPool.ReleaseAll()
}

func (allocator *Allocator) selectPool(idx *big.Int) pool {
	if allocator.autoPool.IsInRange(idx) {
		return allocator.autoPool
	} else {
		return allocator.customPool
	}
}
