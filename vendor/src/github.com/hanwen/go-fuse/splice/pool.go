package splice

import (
	"io"
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

// Return pipe pair to pool
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

func (me *pairPool) clear() {
	me.Lock()
	for _, p := range me.unused {
		p.Close()
	}
	me.unused = me.unused[:0]
	me.Unlock()
}

func (me *pairPool) used() (n int) {
	me.Lock()
	n = me.usedCount
	me.Unlock()

	return n
}

func (me *pairPool) total() int {
	me.Lock()
	n := me.usedCount + len(me.unused)
	me.Unlock()
	return n
}

func (me *pairPool) drop(p *Pair) {
	p.Close()
	me.Lock()
	me.usedCount--
	me.Unlock()
}

func (me *pairPool) get() (p *Pair, err error) {
	me.Lock()
	defer me.Unlock()

	me.usedCount++
	l := len(me.unused)
	if l > 0 {
		p := me.unused[l-1]
		me.unused = me.unused[:l-1]
		return p, nil
	}

	return newSplicePair()
}

var discardBuffer [32 * 1024]byte

func DiscardAll(r io.Reader) {
	buf := discardBuffer[:]
	for {
		n, _ := r.Read(buf)
		if n < len(buf) {
			break
		}
	}
}

func (me *pairPool) done(p *Pair) {
	DiscardAll(p.r)

	me.Lock()
	me.usedCount--
	me.unused = append(me.unused, p)
	me.Unlock()
}

func init() {
	splicePool = newSplicePairPool()
}
