package nodefs

import (
	"log"
	"sync"
)

// HandleMap translates objects in Go space to 64-bit handles that can
// be given out to -say- the linux kernel as NodeIds.
//
// The 32 bits version of this is a threadsafe wrapper around a map.
//
// To use it, include "handled" as first member of the structure
// you wish to export.
//
// This structure is thread-safe.
type handleMap interface {
	// Register stores "obj" and returns a unique (NodeId, generation) tuple.
	Register(obj *handled) (handle, generation uint64)
	Count() int
	// Decode retrieves a stored object from its 64-bit handle.
	Decode(uint64) *handled
	// Forget decrements the reference counter for "handle" by "count" and drops
	// the object if the refcount reaches zero.
	// Returns a boolean whether the object was dropped and the object itself.
	Forget(handle uint64, count int) (bool, *handled)
	// Handle gets the object's NodeId.
	Handle(obj *handled) uint64
	// Has checks if NodeId is stored.
	Has(uint64) bool
}

type handled struct {
	check  uint32
	handle uint64
	count  int
}

func (h *handled) verify() {
	if h.count < 0 {
		log.Panicf("negative lookup count %d", h.count)
	}
	if (h.count == 0) != (h.handle == 0) {
		log.Panicf("registration mismatch: lookup %d id %d", h.count, h.handle)
	}
}

const _ALREADY_MSG = "Object already has a handle"

////////////////////////////////////////////////////////////////
// portable version using 32 bit integers.

type portableHandleMap struct {
	sync.RWMutex
	// The generation counter is incremented each time a NodeId is reused,
	// hence the (NodeId, Generation) tuple is always unique.
	generation uint64
	// Number of currently used handles
	used int
	// Array of Go objects indexed by NodeId
	handles []*handled
	// Free slots in the "handles" array
	freeIds []uint64
}

func newPortableHandleMap() *portableHandleMap {
	return &portableHandleMap{
		// Avoid handing out ID 0 and 1.
		handles: []*handled{nil, nil},
	}
}

func (m *portableHandleMap) Register(obj *handled) (handle, generation uint64) {
	m.Lock()
	if obj.count == 0 {
		if obj.check != 0 {
			panic(_ALREADY_MSG)
		}

		if len(m.freeIds) == 0 {
			handle = uint64(len(m.handles))
			m.handles = append(m.handles, obj)
		} else {
			handle = m.freeIds[len(m.freeIds)-1]
			m.freeIds = m.freeIds[:len(m.freeIds)-1]
			m.generation++
			m.handles[handle] = obj
		}
		m.used++
		obj.handle = handle
	} else {
		handle = obj.handle
	}
	obj.count++
	generation = m.generation
	m.Unlock()
	return
}

func (m *portableHandleMap) Handle(obj *handled) (h uint64) {
	m.RLock()
	if obj.count == 0 {
		h = 0
	} else {
		h = obj.handle
	}
	m.RUnlock()
	return h
}

func (m *portableHandleMap) Count() int {
	m.RLock()
	c := m.used
	m.RUnlock()
	return c
}

func (m *portableHandleMap) Decode(h uint64) *handled {
	m.RLock()
	v := m.handles[h]
	m.RUnlock()
	return v
}

func (m *portableHandleMap) Forget(h uint64, count int) (forgotten bool, obj *handled) {
	m.Lock()
	obj = m.handles[h]
	obj.count -= count
	if obj.count < 0 {
		log.Panicf("underflow: handle %d, count %d, object %d", h, count, obj.count)
	} else if obj.count == 0 {
		m.handles[h] = nil
		m.freeIds = append(m.freeIds, h)
		m.used--
		forgotten = true
		obj.handle = 0
	}
	m.Unlock()
	return forgotten, obj
}

func (m *portableHandleMap) Has(h uint64) bool {
	m.RLock()
	ok := m.handles[h] != nil
	m.RUnlock()
	return ok
}
