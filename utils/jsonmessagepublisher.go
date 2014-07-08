package utils

import (
	"sync"
	"time"
)

func NewJSONMessagePublisher() *JSONMessagePublisher {
	return &JSONMessagePublisher{}
}

type JSONMessageListener chan<- JSONMessage

type JSONMessagePublisher struct {
	m           sync.RWMutex
	subscribers []JSONMessageListener
}

func (p *JSONMessagePublisher) Subscribe(l JSONMessageListener) {
	p.m.Lock()
	p.subscribers = append(p.subscribers, l)
	p.m.Unlock()
}

func (p *JSONMessagePublisher) SubscribersCount() int {
	p.m.RLock()
	count := len(p.subscribers)
	p.m.RUnlock()
	return count
}

// Unsubscribe closes and removes the specified listener from the list of
// previously registed ones.
// It returns a boolean value indicating if the listener was successfully
// found, closed and unregistered.
func (p *JSONMessagePublisher) Unsubscribe(l JSONMessageListener) bool {
	p.m.Lock()
	defer p.m.Unlock()

	for i, subscriber := range p.subscribers {
		if subscriber == l {
			close(l)
			p.subscribers = append(p.subscribers[:i], p.subscribers[i+1:]...)
			return true
		}
	}
	return false
}

func (p *JSONMessagePublisher) Publish(m JSONMessage) {
	p.m.RLock()
	for _, subscriber := range p.subscribers {
		// We give each subscriber a 100ms time window to receive the event,
		// after which we move to the next.
		select {
		case subscriber <- m:
		case <-time.After(100 * time.Millisecond):
		}
	}
	p.m.RUnlock()
}
