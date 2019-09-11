// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package splice

import (
	"sync"
)

var splicePool *pairPool

type pairPool struct {
	sync.Mutex
	unused    []*Pair
	usedCount int
}

func ClearSplicePool() {
	splicePool.clear()
}

func Get() (*Pair, error) {
	return splicePool.get()
}

func Total() int {
	return splicePool.total()
}

func Used() int {
	return splicePool.used()
}

// Done returns the pipe pair to pool.
func Done(p *Pair) {
	splicePool.done(p)
}

// Closes and discards pipe pair.
func Drop(p *Pair) {
	splicePool.drop(p)
}

func newSplicePairPool() *pairPool {
	return &pairPool{}
}

func (pp *pairPool) clear() {
	pp.Lock()
	for _, p := range pp.unused {
		p.Close()
	}
	pp.unused = pp.unused[:0]
	pp.Unlock()
}

func (pp *pairPool) used() (n int) {
	pp.Lock()
	n = pp.usedCount
	pp.Unlock()

	return n
}

func (pp *pairPool) total() int {
	pp.Lock()
	n := pp.usedCount + len(pp.unused)
	pp.Unlock()
	return n
}

func (pp *pairPool) drop(p *Pair) {
	p.Close()
	pp.Lock()
	pp.usedCount--
	pp.Unlock()
}

func (pp *pairPool) get() (p *Pair, err error) {
	pp.Lock()
	defer pp.Unlock()

	pp.usedCount++
	l := len(pp.unused)
	if l > 0 {
		p := pp.unused[l-1]
		pp.unused = pp.unused[:l-1]
		return p, nil
	}

	return newSplicePair()
}

func (pp *pairPool) done(p *Pair) {
	p.discard()

	pp.Lock()
	pp.usedCount--
	pp.unused = append(pp.unused, p)
	pp.Unlock()
}

func init() {
	splicePool = newSplicePairPool()
}
