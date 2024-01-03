/*
Open Source Initiative OSI - The MIT License (MIT):Licensing

The MIT License (MIT)
Copyright (c) 2013 - 2022 Ralph Caraveo (deckarep@gmail.com)

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

import "sync"

type threadSafeSet[T comparable] struct {
	sync.RWMutex
	uss threadUnsafeSet[T]
}

func newThreadSafeSet[T comparable]() *threadSafeSet[T] {
	return &threadSafeSet[T]{
		uss: newThreadUnsafeSet[T](),
	}
}

func newThreadSafeSetWithSize[T comparable](cardinality int) *threadSafeSet[T] {
	return &threadSafeSet[T]{
		uss: newThreadUnsafeSetWithSize[T](cardinality),
	}
}

func (t *threadSafeSet[T]) Add(v T) bool {
	t.Lock()
	ret := t.uss.Add(v)
	t.Unlock()
	return ret
}

func (t *threadSafeSet[T]) Append(v ...T) int {
	t.Lock()
	ret := t.uss.Append(v...)
	t.Unlock()
	return ret
}

func (t *threadSafeSet[T]) Contains(v ...T) bool {
	t.RLock()
	ret := t.uss.Contains(v...)
	t.RUnlock()

	return ret
}

func (t *threadSafeSet[T]) IsSubset(other Set[T]) bool {
	o := other.(*threadSafeSet[T])

	t.RLock()
	o.RLock()

	ret := t.uss.IsSubset(o.uss)
	t.RUnlock()
	o.RUnlock()
	return ret
}

func (t *threadSafeSet[T]) IsProperSubset(other Set[T]) bool {
	o := other.(*threadSafeSet[T])

	t.RLock()
	defer t.RUnlock()
	o.RLock()
	defer o.RUnlock()

	return t.uss.IsProperSubset(o.uss)
}

func (t *threadSafeSet[T]) IsSuperset(other Set[T]) bool {
	return other.IsSubset(t)
}

func (t *threadSafeSet[T]) IsProperSuperset(other Set[T]) bool {
	return other.IsProperSubset(t)
}

func (t *threadSafeSet[T]) Union(other Set[T]) Set[T] {
	o := other.(*threadSafeSet[T])

	t.RLock()
	o.RLock()

	unsafeUnion := t.uss.Union(o.uss).(threadUnsafeSet[T])
	ret := &threadSafeSet[T]{uss: unsafeUnion}
	t.RUnlock()
	o.RUnlock()
	return ret
}

func (t *threadSafeSet[T]) Intersect(other Set[T]) Set[T] {
	o := other.(*threadSafeSet[T])

	t.RLock()
	o.RLock()

	unsafeIntersection := t.uss.Intersect(o.uss).(threadUnsafeSet[T])
	ret := &threadSafeSet[T]{uss: unsafeIntersection}
	t.RUnlock()
	o.RUnlock()
	return ret
}

func (t *threadSafeSet[T]) Difference(other Set[T]) Set[T] {
	o := other.(*threadSafeSet[T])

	t.RLock()
	o.RLock()

	unsafeDifference := t.uss.Difference(o.uss).(threadUnsafeSet[T])
	ret := &threadSafeSet[T]{uss: unsafeDifference}
	t.RUnlock()
	o.RUnlock()
	return ret
}

func (t *threadSafeSet[T]) SymmetricDifference(other Set[T]) Set[T] {
	o := other.(*threadSafeSet[T])

	t.RLock()
	o.RLock()

	unsafeDifference := t.uss.SymmetricDifference(o.uss).(threadUnsafeSet[T])
	ret := &threadSafeSet[T]{uss: unsafeDifference}
	t.RUnlock()
	o.RUnlock()
	return ret
}

func (t *threadSafeSet[T]) Clear() {
	t.Lock()
	t.uss.Clear()
	t.Unlock()
}

func (t *threadSafeSet[T]) Remove(v T) {
	t.Lock()
	delete(t.uss, v)
	t.Unlock()
}

func (t *threadSafeSet[T]) RemoveAll(i ...T) {
	t.Lock()
	t.uss.RemoveAll(i...)
	t.Unlock()
}

func (t *threadSafeSet[T]) Cardinality() int {
	t.RLock()
	defer t.RUnlock()
	return len(t.uss)
}

func (t *threadSafeSet[T]) Each(cb func(T) bool) {
	t.RLock()
	for elem := range t.uss {
		if cb(elem) {
			break
		}
	}
	t.RUnlock()
}

func (t *threadSafeSet[T]) Iter() <-chan T {
	ch := make(chan T)
	go func() {
		t.RLock()

		for elem := range t.uss {
			ch <- elem
		}
		close(ch)
		t.RUnlock()
	}()

	return ch
}

func (t *threadSafeSet[T]) Iterator() *Iterator[T] {
	iterator, ch, stopCh := newIterator[T]()

	go func() {
		t.RLock()
	L:
		for elem := range t.uss {
			select {
			case <-stopCh:
				break L
			case ch <- elem:
			}
		}
		close(ch)
		t.RUnlock()
	}()

	return iterator
}

func (t *threadSafeSet[T]) Equal(other Set[T]) bool {
	o := other.(*threadSafeSet[T])

	t.RLock()
	o.RLock()

	ret := t.uss.Equal(o.uss)
	t.RUnlock()
	o.RUnlock()
	return ret
}

func (t *threadSafeSet[T]) Clone() Set[T] {
	t.RLock()

	unsafeClone := t.uss.Clone().(threadUnsafeSet[T])
	ret := &threadSafeSet[T]{uss: unsafeClone}
	t.RUnlock()
	return ret
}

func (t *threadSafeSet[T]) String() string {
	t.RLock()
	ret := t.uss.String()
	t.RUnlock()
	return ret
}

func (t *threadSafeSet[T]) Pop() (T, bool) {
	t.Lock()
	defer t.Unlock()
	return t.uss.Pop()
}

func (t *threadSafeSet[T]) ToSlice() []T {
	keys := make([]T, 0, t.Cardinality())
	t.RLock()
	for elem := range t.uss {
		keys = append(keys, elem)
	}
	t.RUnlock()
	return keys
}

func (t *threadSafeSet[T]) MarshalJSON() ([]byte, error) {
	t.RLock()
	b, err := t.uss.MarshalJSON()
	t.RUnlock()

	return b, err
}

func (t *threadSafeSet[T]) UnmarshalJSON(p []byte) error {
	t.RLock()
	err := t.uss.UnmarshalJSON(p)
	t.RUnlock()

	return err
}
