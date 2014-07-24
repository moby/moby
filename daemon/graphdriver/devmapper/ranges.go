// +build linux,amd64

package devmapper

import (
	"container/list"
	"fmt"
)

type Range struct {
	begin uint64
	end   uint64
}

type Ranges struct {
	*list.List
}

func NewRanges() *Ranges {
	return &Ranges{list.New()}
}

func (r *Ranges) ToString() string {
	s := ""
	for e := r.Front(); e != nil; e = e.Next() {
		r := e.Value.(*Range)
		if s != "" {
			s = s + ","
		}
		s = fmt.Sprintf("%s%d-%d", s, r.begin, r.end)
	}
	return s
}

func (r *Ranges) Clear() {
	r.Init()
}

func (r *Ranges) Add(begin, end uint64) {
	var next *list.Element
	for e := r.Front(); e != nil; e = next {
		next = e.Next()

		existing := e.Value.(*Range)

		// If existing range is fully to the left, skip
		if existing.end < begin {
			continue
		}

		// If new range is fully to the left, just insert
		if end < existing.begin {
			r.InsertBefore(&Range{begin, end}, e)
			return
		}

		// Now we know the two ranges somehow intersect (or at least touch)

		// Extend existing range with the new range
		if begin < existing.begin {
			existing.begin = begin
		}

		// If the new range is completely covered by existing range, we're done
		if end <= existing.end {
			return
		}

		// Otherwise strip r from new range
		begin = existing.end

		// We're now touching r at the end, and so we need to either extend r
		// or merge with next

		if next == nil {
			// Nothing after, extend
			existing.end = end
			return
		}

		nextR := next.Value.(*Range)
		if end < nextR.begin {
			// Fits, Just extend
			existing.end = end
			return
		}

		// The new region overlaps the next, merge the two
		nextR.begin = existing.begin
		r.Remove(e)
	}

	// nothing in list or everything to the left, just append the rest
	if begin < end {
		r.PushBack(&Range{begin, end})
		return
	}
}
