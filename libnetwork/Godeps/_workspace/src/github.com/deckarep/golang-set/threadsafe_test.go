/*
Open Source Initiative OSI - The MIT License (MIT):Licensing

The MIT License (MIT)
Copyright (c) 2013 Ralph Caraveo (deckarep@gmail.com)

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
of the Software, and to permit persons to whom the Software is furnished to do
so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package mapset

import (
	"math/rand"
	"runtime"
	"sync"
	"testing"
)

const N = 1000

func Test_AddConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSet()
	ints := rand.Perm(N)

	var wg sync.WaitGroup
	wg.Add(len(ints))
	for i := 0; i < len(ints); i++ {
		go func(i int) {
			s.Add(i)
			wg.Done()
		}(i)
	}

	wg.Wait()
	for _, i := range ints {
		if !s.Contains(i) {
			t.Errorf("Set is missing element: %v", i)
		}
	}
}

func Test_CardinalityConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSet()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		elems := s.Cardinality()
		for i := 0; i < N; i++ {
			newElems := s.Cardinality()
			if newElems < elems {
				t.Errorf("Cardinality shrunk from %v to %v", elems, newElems)
			}
		}
		wg.Done()
	}()

	for i := 0; i < N; i++ {
		s.Add(rand.Int())
	}
	wg.Wait()
}

func Test_ClearConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSet()
	ints := rand.Perm(N)

	var wg sync.WaitGroup
	wg.Add(len(ints))
	for i := 0; i < len(ints); i++ {
		go func() {
			s.Clear()
			wg.Done()
		}()
		go func(i int) {
			s.Add(i)
		}(i)
	}

	wg.Wait()
}

func Test_CloneConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSet()
	ints := rand.Perm(N)

	for _, v := range ints {
		s.Add(v)
	}

	var wg sync.WaitGroup
	wg.Add(len(ints))
	for i := range ints {
		go func(i int) {
			s.Remove(i)
			wg.Done()
		}(i)
	}

	s.Clone()
}

func Test_ContainsConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSet()
	ints := rand.Perm(N)
	interfaces := make([]interface{}, 0)
	for _, v := range ints {
		s.Add(v)
		interfaces = append(interfaces, v)
	}

	var wg sync.WaitGroup
	for _ = range ints {
		go func() {
			s.Contains(interfaces...)
		}()
	}
	wg.Wait()
}

func Test_DifferenceConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s, ss := NewSet(), NewSet()
	ints := rand.Perm(N)
	interfaces := make([]interface{}, 0)
	for _, v := range ints {
		s.Add(v)
		ss.Add(v)
		interfaces = append(interfaces, v)
	}

	var wg sync.WaitGroup
	for _ = range ints {
		go func() {
			s.Difference(ss)
		}()
	}
	wg.Wait()
}

func Test_EqualConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s, ss := NewSet(), NewSet()
	ints := rand.Perm(N)
	interfaces := make([]interface{}, 0)
	for _, v := range ints {
		s.Add(v)
		ss.Add(v)
		interfaces = append(interfaces, v)
	}

	var wg sync.WaitGroup
	for _ = range ints {
		go func() {
			s.Equal(ss)
		}()
	}
	wg.Wait()
}

func Test_IntersectConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s, ss := NewSet(), NewSet()
	ints := rand.Perm(N)
	interfaces := make([]interface{}, 0)
	for _, v := range ints {
		s.Add(v)
		ss.Add(v)
		interfaces = append(interfaces, v)
	}

	var wg sync.WaitGroup
	for _ = range ints {
		go func() {
			s.Intersect(ss)
		}()
	}
	wg.Wait()
}

func Test_IsSubsetConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s, ss := NewSet(), NewSet()
	ints := rand.Perm(N)
	interfaces := make([]interface{}, 0)
	for _, v := range ints {
		s.Add(v)
		ss.Add(v)
		interfaces = append(interfaces, v)
	}

	var wg sync.WaitGroup
	for _ = range ints {
		go func() {
			s.IsSubset(ss)
		}()
	}
	wg.Wait()
}

func Test_IsSupersetConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s, ss := NewSet(), NewSet()
	ints := rand.Perm(N)
	interfaces := make([]interface{}, 0)
	for _, v := range ints {
		s.Add(v)
		ss.Add(v)
		interfaces = append(interfaces, v)
	}

	var wg sync.WaitGroup
	for _ = range ints {
		go func() {
			s.IsSuperset(ss)
		}()
	}
	wg.Wait()
}

func Test_IterConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSet()
	ints := rand.Perm(N)
	for _, v := range ints {
		s.Add(v)
	}

	cs := make([]<-chan interface{}, 0)
	for _ = range ints {
		cs = append(cs, s.Iter())
	}

	c := make(chan interface{})
	go func() {
		for n := 0; n < len(ints)*N; {
			for _, d := range cs {
				select {
				case <-d:
					n++
					c <- nil
				default:
				}
			}
		}
		close(c)
	}()

	for _ = range c {
	}
}

func Test_RemoveConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSet()
	ints := rand.Perm(N)
	for _, v := range ints {
		s.Add(v)
	}

	var wg sync.WaitGroup
	wg.Add(len(ints))
	for _, v := range ints {
		go func(i int) {
			s.Remove(i)
			wg.Done()
		}(v)
	}
	wg.Wait()

	if s.Cardinality() != 0 {
		t.Errorf("Expected cardinality 0; got %v", s.Cardinality())
	}
}

func Test_StringConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSet()
	ints := rand.Perm(N)
	for _, v := range ints {
		s.Add(v)
	}

	var wg sync.WaitGroup
	wg.Add(len(ints))
	for _ = range ints {
		go func() {
			s.String()
			wg.Done()
		}()
	}
	wg.Wait()
}

func Test_SymmetricDifferenceConcurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s, ss := NewSet(), NewSet()
	ints := rand.Perm(N)
	interfaces := make([]interface{}, 0)
	for _, v := range ints {
		s.Add(v)
		ss.Add(v)
		interfaces = append(interfaces, v)
	}

	var wg sync.WaitGroup
	for _ = range ints {
		go func() {
			s.SymmetricDifference(ss)
		}()
	}
	wg.Wait()
}

func Test_ToSlice(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSet()
	ints := rand.Perm(N)

	var wg sync.WaitGroup
	wg.Add(len(ints))
	for i := 0; i < len(ints); i++ {
		go func(i int) {
			s.Add(i)
			wg.Done()
		}(i)
	}

	wg.Wait()
	setAsSlice := s.ToSlice()
	if len(setAsSlice) != s.Cardinality() {
		t.Errorf("Set length is incorrect: %v", len(setAsSlice))
	}

	for _, i := range setAsSlice {
		if !s.Contains(i) {
			t.Errorf("Set is missing element: %v", i)
		}
	}
}
