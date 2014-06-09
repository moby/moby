// +build linux,amd64

package devmapper

import (
	"fmt"
	"testing"
)

func assert(t *testing.T, r *Ranges, res string) {
	s := r.ToString()
	if s != res {
		t.Fatalf(fmt.Sprintf("error: got %s, expecting %s\n", s, res))
	}
}

func TestRanges(t *testing.T) {
	r := NewRanges()
	assert(t, r, "")
	r.Clear()
	assert(t, r, "")
	r.Add(5, 6)
	assert(t, r, "5-6")
	r.Add(5, 6)
	assert(t, r, "5-6")
	r.Add(5, 7)
	assert(t, r, "5-7")
	r.Add(7, 8)
	assert(t, r, "5-8")
	r.Add(4, 6)
	assert(t, r, "4-8")
	r.Add(5, 6)
	assert(t, r, "4-8")
	r.Add(3, 4)
	assert(t, r, "3-8")
	r.Add(1, 2)
	assert(t, r, "1-2,3-8")
	r.Add(15, 20)
	assert(t, r, "1-2,3-8,15-20")
	r.Add(30, 40)
	assert(t, r, "1-2,3-8,15-20,30-40")
	r.Add(8, 9)
	assert(t, r, "1-2,3-9,15-20,30-40")
	r.Add(8, 10)
	assert(t, r, "1-2,3-10,15-20,30-40")
	r.Add(8, 25)
	assert(t, r, "1-2,3-25,30-40")
	r.Add(0, 27)
	assert(t, r, "0-27,30-40")
	r.Add(29, 41)
	assert(t, r, "0-27,29-41")
	r.Add(27, 29)
	assert(t, r, "0-41")
}
