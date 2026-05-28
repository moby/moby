package history

import "sync"

type pubsub[T any] struct {
	mu sync.Mutex
	m  map[*channel[T]]struct{}
}

func (p *pubsub[T]) Subscribe() *channel[T] {
	p.mu.Lock()
	c := &channel[T]{
		ps:   p,
		ch:   make(chan T, 32),
		done: make(chan struct{}),
	}
	p.m[c] = struct{}{}
	p.mu.Unlock()
	return c
}

func (p *pubsub[T]) Send(v T) {
	p.mu.Lock()
	for c := range p.m {
		go c.send(v)
	}
	p.mu.Unlock()
}

func (p *pubsub[T]) Close() {
	p.mu.Lock()
	channels := make([]*channel[T], 0, len(p.m))
	for c := range p.m {
		channels = append(channels, c)
	}
	p.mu.Unlock()
	for _, c := range channels {
		c.close()
	}
}

type channel[T any] struct {
	ps        *pubsub[T]
	ch        chan T
	done      chan struct{}
	closeOnce sync.Once
}

func (p *channel[T]) send(v T) {
	select {
	case p.ch <- v:
	case <-p.done:
	}
}

func (p *channel[T]) close() {
	p.closeOnce.Do(func() {
		p.ps.mu.Lock()
		delete(p.ps.m, p)
		p.ps.mu.Unlock()
		close(p.done)
	})
}
