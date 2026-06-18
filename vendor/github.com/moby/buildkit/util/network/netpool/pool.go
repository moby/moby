package netpool

import (
	"context"
	"sync"
	"time"

	"github.com/moby/buildkit/util/bklog"
	"github.com/pkg/errors"
)

const aboveTargetGracePeriod = 5 * time.Minute

type Opt[T any] struct {
	Name       string
	TargetSize int
	New        func(context.Context) (T, error)
	Release    func(T) error
}

type Pool[T any] struct {
	name       string
	targetSize int
	new        func(context.Context) (T, error)
	release    func(T) error

	mu         sync.Mutex
	actualSize int
	available  []pooled[T]
	closed     bool
}

type pooled[T any] struct {
	value    T
	lastUsed time.Time
}

func New[T any](opt Opt[T]) *Pool[T] {
	name := opt.Name
	if name == "" {
		name = "network namespace"
	}
	return &Pool[T]{
		name:       name,
		targetSize: opt.TargetSize,
		new:        opt.New,
		release:    opt.Release,
	}
}

func (p *Pool[T]) Close() error {
	bklog.L.Debugf("cleaning up %s pool", p.name)

	p.mu.Lock()
	p.closed = true
	available := p.available
	p.available = nil
	p.actualSize -= len(available)
	p.mu.Unlock()

	var err error
	for _, v := range available {
		if e := p.release(v.value); e != nil && err == nil {
			err = e
		}
	}
	return err
}

func (p *Pool[T]) Fill(ctx context.Context) {
	for {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}
		actualSize := p.actualSize
		p.mu.Unlock()
		if actualSize >= p.targetSize {
			return
		}
		v, err := p.getNew(ctx)
		if err != nil {
			bklog.G(ctx).Errorf("failed to create new %s while prefilling pool: %s", p.name, err)
			return
		}
		p.Put(v)
	}
}

func (p *Pool[T]) Get(ctx context.Context) (T, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		var zero T
		return zero, errors.Errorf("%s pool is closed", p.name)
	}
	if len(p.available) > 0 {
		v := p.available[len(p.available)-1].value
		p.available = p.available[:len(p.available)-1]
		p.mu.Unlock()
		return v, nil
	}
	p.mu.Unlock()

	return p.getNew(ctx)
}

func (p *Pool[T]) Put(v T) {
	putTime := time.Now()

	p.mu.Lock()
	if p.closed {
		p.actualSize--
		p.mu.Unlock()
		_ = p.release(v)
		return
	}
	p.available = append(p.available, pooled[T]{value: v, lastUsed: putTime})
	actualSize := p.actualSize
	p.mu.Unlock()

	if actualSize > p.targetSize {
		time.AfterFunc(aboveTargetGracePeriod, p.cleanupToTargetSize)
	}
}

func (p *Pool[T]) Discard(v T) error {
	p.mu.Lock()
	p.actualSize--
	p.mu.Unlock()
	return p.release(v)
}

func (p *Pool[T]) getNew(ctx context.Context) (T, error) {
	v, err := p.new(ctx)
	if err != nil {
		return v, err
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		if e := p.release(v); e != nil {
			return v, e
		}
		var zero T
		return zero, errors.Errorf("%s pool is closed", p.name)
	}
	p.actualSize++
	p.mu.Unlock()
	return v, nil
}

func (p *Pool[T]) cleanupToTargetSize() {
	var toRelease []T
	defer func() {
		for _, v := range toRelease {
			_ = p.release(v)
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	for p.actualSize > p.targetSize &&
		len(p.available) > 0 &&
		time.Since(p.available[0].lastUsed) >= aboveTargetGracePeriod {
		toRelease = append(toRelease, p.available[0].value)
		p.available = p.available[1:]
		p.actualSize--
	}
}
