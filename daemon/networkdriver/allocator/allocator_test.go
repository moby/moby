package allocator

import (
	"math/big"
	"testing"
)

func allocateRange(alloc *Allocator, start, end *big.Int, t *testing.T) {
	for i := big.NewInt(0).Set(start); i.Cmp(end) < 0; i.Add(i, big.NewInt(1)) {
		idx, err := alloc.AllocateFirstAvailable()
		if err != nil {
			t.Fatalf("Expected index %d, got error %v", i, err)
		} else if i.Cmp(idx) != 0 {
			t.Fatalf("Expected index %d, got %d", i, idx)
		}
	}
}

func TestAllocateAuto(t *testing.T) {
	alloc := NewAllocator(big.NewInt(0), big.NewInt(100))
	allocateRange(alloc, big.NewInt(0), big.NewInt(0).Div(alloc.AutomaticPoolSize(), big.NewInt(2)), t)
}

func TestReallocateAuto(t *testing.T) {
	alloc := NewAllocator(big.NewInt(100), big.NewInt(200))
	idx, err := alloc.AllocateFirstAvailable()
	if err != nil {
		t.Fatalf("Failed to allocate automatic slot: %v", err)
	}
	if err := alloc.Allocate(idx); err == nil {
		t.Fatalf("Slot was allocated twice")
	}
}

func TestAllocatorRangeSize(t *testing.T) {
	var low, high int64

	low, high = 0, 10
	alloc := NewAllocator(big.NewInt(low), big.NewInt(high))
	if expected := big.NewInt(high - low + 1); alloc.AutomaticPoolSize().Cmp(expected) != 0 {
		t.Fatalf("Wrong allocator range size: expected %d, got %d", expected, alloc.AutomaticPoolSize())
	}

	low, high = 100, 200
	alloc = NewAllocator(big.NewInt(low), big.NewInt(high))
	if expected := big.NewInt(high - low + 1); alloc.AutomaticPoolSize().Cmp(expected) != 0 {
		t.Fatalf("Wrong allocator range size: expected %d, got %d", expected, alloc.AutomaticPoolSize())
	}

	low, high = 100, 100
	alloc = NewAllocator(big.NewInt(low), big.NewInt(high))
	if expected := big.NewInt(1); alloc.AutomaticPoolSize().Cmp(expected) != 0 {
		t.Fatalf("Wrong allocator range size: expected %d, got %d", expected, alloc.AutomaticPoolSize())
	}
}

func TestAllocateExplicit(t *testing.T) {
	alloc := NewAllocator(big.NewInt(100), big.NewInt(200))
	if err := alloc.Allocate(big.NewInt(10)); err != nil {
		t.Fatalf("Failed to allocate explicit slot: %v", err)
	}
	if err := alloc.Allocate(big.NewInt(10)); err == nil {
		t.Fatalf("Slot was allocated twice")
	}

	if err := alloc.Allocate(big.NewInt(100)); err != nil {
		t.Fatalf("Failed to allocate explicit slot: %v", err)
	}
	if err := alloc.Allocate(big.NewInt(100)); err == nil {
		t.Fatalf("Slot was allocated twice")
	}

	expected := big.NewInt(101)
	idx, err := alloc.AllocateFirstAvailable()
	if err != nil {
		t.Fatalf("Expected index %d, got error %v", expected, err)
	} else if idx.Cmp(expected) != 0 {
		t.Fatalf("Expected index %d, got %d", expected, idx)
	}
}

func TestAllocateWholeRange(t *testing.T) {
	alloc := NewAllocator(big.NewInt(0), big.NewInt(10))
	allocateRange(alloc, big.NewInt(0), alloc.AutomaticPoolSize(), t)
	if idx, err := alloc.AllocateFirstAvailable(); err == nil {
		t.Fatalf("Expected error %v, got index %d", err, idx)
	}
}

func TestReuseReleased(t *testing.T) {
	alloc := NewAllocator(big.NewInt(0), big.NewInt(10))
	allocateRange(alloc, big.NewInt(0), alloc.AutomaticPoolSize(), t)

	var slot int64 = 5
	alloc.Release(big.NewInt(slot))
	idx, err := alloc.AllocateFirstAvailable()
	if err != nil {
		t.Fatalf("Expected index %d, got error %v", slot, err)
	} else if idx.Cmp(big.NewInt(slot)) != 0 {
		t.Fatalf("Expected index %d, got %d", slot, idx)
	}

	slot = 8
	alloc.Release(big.NewInt(slot))
	idx, err = alloc.AllocateFirstAvailable()
	if err != nil {
		t.Fatalf("Expected index %d, got error %v", slot, err)
	} else if idx.Cmp(big.NewInt(slot)) != 0 {
		t.Fatalf("Expected index %d, got %d", slot, idx)
	}
}
