package serde

import "unsafe"

// ArenaPayloadFactor is the ratio at which we size string arenas to the payload
// (clamped by StringArenaMin/Max)
//
// We limit arenas to 4k but smaller payloads likely don't need it, so we size
// down (somewhat arbitrarily) until we hit the 512-byte floor at which we just
// don't arena.
const ArenaPayloadFactor = 4

// The size of a string arena is bounded, but not strictly by [min, max]:
//   - Attempting to set a capacity below MinArenaSize will have no effect as
//     beneath this limit it is not worth it. Calls to String with a capacity below
//     this boundary will simply allocate.
//   - An arena WILL be capped at MaxArenaSize.
const (
	MinArenaSize = 512
	MaxArenaSize = 4096
)

// StringArena batches many small string allocations into one block.
//
// You MUST call Reset with the desired capacity before use. Failure to do so
// will mean any calls to String will simply allocate and you won't get the
// performance benefit of the arena.
type StringArena struct {
	buf []byte
	cap int
}

// Reset prepares the arena for a new document, sizing its next block to cap.
func (a *StringArena) Reset(cap int) {
	if cap > MaxArenaSize {
		cap = MaxArenaSize
	}
	a.buf = nil
	a.cap = cap
}

// String attempts to intern a copy of b in the arena, returning a string slice
// for it.
//
// If the arena is at capacity, it will just allocate a new string.
func (a *StringArena) String(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	if a.buf == nil { // lazy init, there might be no strings at all
		if a.cap < MinArenaSize || len(b) > a.cap {
			return string(b) // "not worth it" fallback
		}

		a.buf = make([]byte, 0, a.cap)
	}

	if len(b) > cap(a.buf)-len(a.buf) {
		return string(b) // "over budget" fallback
	}

	off := len(a.buf)
	a.buf = append(a.buf, b...)
	return unsafe.String(&a.buf[off], len(b))
}
