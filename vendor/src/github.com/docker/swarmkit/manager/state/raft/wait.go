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
	// callback which is called to cancel a waiter
	cancel func()
}

type wait struct {
	l sync.Mutex
	m map[uint64]waitItem
}

func newWait() *wait {
	return &wait{m: make(map[uint64]waitItem)}
}

func (w *wait) register(id uint64, cb func(), cancel func()) <-chan interface{} {
	w.l.Lock()
	defer w.l.Unlock()
	_, ok := w.m[id]
	if !ok {
		ch := make(chan interface{}, 1)
		w.m[id] = waitItem{ch: ch, cb: cb, cancel: cancel}
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
		return true
	}
	return false
}

func (w *wait) cancel(id uint64) {
	w.l.Lock()
	waitItem, ok := w.m[id]
	delete(w.m, id)
	w.l.Unlock()
	if ok && waitItem.cancel != nil {
		waitItem.cancel()
	}
}

func (w *wait) cancelAll() {
	w.l.Lock()
	defer w.l.Unlock()

	for id, waitItem := range w.m {
		delete(w.m, id)
		if waitItem.cancel != nil {
			waitItem.cancel()
		}
	}
}
