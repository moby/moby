package network

import "sync"

type nameLocker struct {
	mutex  sync.Mutex
	muxMap map[string]chan bool
}

func (n *nameLocker) init() {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if n.muxMap == nil {
		n.muxMap = make(map[string]chan bool)
	}
}

// Each goroutine that reaches this function races to create a named entry in
// the protected map. The first one per name to succeed will:
// 1. create the named entry
// 2. create a channel bound to this entry
// 3. continue to exit the function and do its thing
//
// The other (per name) will wait on the channel, until the first goroutine
// calls unlock (which removes the named entry and closes the channel, releasing
// all). They will now race to be the next to recreate the entry/channel
func (n *nameLocker) lock(name string) chan bool {
	acquired := false
	for !acquired {
		n.mutex.Lock()
		if n.muxMap[name] == nil {
			n.muxMap[name] = make(chan bool)
			acquired = true
		}
		n.mutex.Unlock()
		if !acquired {
			_ = <-n.muxMap[name]
		}
	}
	return n.muxMap[name]
}

// the goroutine that reaches unlock makes sure that the entry is removed, only
// then releaseing the rest that wait on this channel
func (n *nameLocker) unlock(name string, c chan bool) {
	n.mutex.Lock()
	if n.muxMap[name] != nil {
		delete(n.muxMap, name)
	}
	n.mutex.Unlock()
	close(c)
}
