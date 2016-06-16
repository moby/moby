package raft

import (
	"fmt"
	"sync"
)

type waitItem struct {
	// channel to wait up the waiter
	ch chan interface{}
	// callback which is called synchronously when the wait is triggered
	cb func()
}

type wait struct {
	l sync.Mutex
	m map[uint64]waitItem
}

func newWait() *wait {
	return &wait{m: make(map[uint64]waitItem)}
}

func (w *wait) register(id uint64, cb func()) <-chan interface{} {
	w.l.Lock()
	defer w.l.Unlock()
	_, ok := w.m[id]
	if !ok {
		ch := make(chan interface{}, 1)
		w.m[id] = waitItem{ch: ch, cb: cb}
		return ch
	}
	panic(fmt.Sprintf("duplicate id %x", id))
}

func (w *wait) trigger(id uint64, x interface{}) bool {
	w.l.Lock()
	waitItem, ok := w.m[id]
	delete(w.m, id)
	w.l.Unlock()
	if ok {
		if waitItem.cb != nil {
			waitItem.cb()
		}
		waitItem.ch <- x
		close(waitItem.ch)
		return true
	}
	return false
}

func (w *wait) cancel(id uint64) {
	w.l.Lock()
	waitItem, ok := w.m[id]
	delete(w.m, id)
	w.l.Unlock()
	if ok {
		close(waitItem.ch)
	}
}

func (w *wait) cancelAll() {
	w.l.Lock()
	defer w.l.Unlock()

	for id, waitItem := range w.m {
		delete(w.m, id)
		close(waitItem.ch)
	}
}
