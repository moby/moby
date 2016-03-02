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

func (n *nameLocker) unlock(name string, c chan bool) {
	n.mutex.Lock()
	if n.muxMap[name] != nil {
		delete(n.muxMap, name)
	}
	n.mutex.Unlock()
	close(c)
}
