package foreground

import (
	"sync"
)

type Refcount struct {
	count int
	lock  sync.Mutex
	cond  *sync.Cond
}

func NewRefcount() *Refcount {
	r := &Refcount{}
	r.cond = sync.NewCond(&r.lock)
	return r
}

func (r *Refcount) Ref() {
	r.lock.Lock()
	r.count++
	r.lock.Unlock()
}

func (r *Refcount) Unref() {
	r.lock.Lock()
	r.count--
	r.cond.Broadcast()
	r.lock.Unlock()
}

// Wait until the reference count is zero. Note that it may reach zero
// and then increase before this function returns. However, at least
// it is guaranteed that all Refs called before the wait are Unrefed,
// which is all we need.
func (r *Refcount) WaitForZero() {
	r.lock.Lock()
	for r.count != 0 {
		r.cond.Wait()
	}
	r.lock.Unlock()
}
