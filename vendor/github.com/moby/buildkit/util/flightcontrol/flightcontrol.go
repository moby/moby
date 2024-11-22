package flightcontrol

import (
	"context"
	"io"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/moby/buildkit/util/progress"
	"github.com/pkg/errors"
)

// flightcontrol is like singleflight but with support for cancellation and
// nested progress reporting

var (
	errRetry        = errors.Errorf("retry")
	errRetryTimeout = errors.Errorf("exceeded retry timeout")
)

type contextKeyT string

var contextKey = contextKeyT("buildkit/util/flightcontrol.progress")

// Group is a flightcontrol synchronization group
type Group[T any] struct {
	mu sync.Mutex          // protects m
	m  map[string]*call[T] // lazily initialized
}

// Do executes a context function syncronized by the key
func (g *Group[T]) Do(ctx context.Context, key string, fn func(ctx context.Context) (T, error)) (v T, err error) {
	var backoff time.Duration
	for {
		v, err = g.do(ctx, key, fn)
		if err == nil || !errors.Is(err, errRetry) {
			return v, err
		}
		// backoff logic
		if backoff >= 15*time.Second {
			err = errors.Wrapf(errRetryTimeout, "flightcontrol")
			return v, err
		}
		if backoff > 0 {
			backoff = time.Duration(float64(backoff) * 1.2)
		} else {
			// randomize initial backoff to avoid all goroutines retrying at once
			//nolint:gosec // using math/rand pseudo-randomness is acceptable here
			backoff = time.Millisecond + time.Duration(rand.Intn(1e7))*time.Nanosecond
		}
		time.Sleep(backoff)
	}
}

func (g *Group[T]) do(ctx context.Context, key string, fn func(ctx context.Context) (T, error)) (T, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call[T])
	}

	if c, ok := g.m[key]; ok { // register 2nd waiter
		g.mu.Unlock()
		return c.wait(ctx)
	}

	c := newCall(fn)
	g.m[key] = c
	go func() {
		// cleanup after a caller has returned
		<-c.ready
		g.mu.Lock()
		delete(g.m, key)
		g.mu.Unlock()
		close(c.cleaned)
	}()
	g.mu.Unlock()
	return c.wait(ctx)
}

type call[T any] struct {
	mu      sync.Mutex
	result  T
	err     error
	ready   chan struct{}
	cleaned chan struct{}

	ctx  *sharedContext[T]
	ctxs []context.Context
	fn   func(ctx context.Context) (T, error)
	once sync.Once

	closeProgressWriter func(error)
	progressState       *progressState
	progressCtx         context.Context
}

func newCall[T any](fn func(ctx context.Context) (T, error)) *call[T] {
	c := &call[T]{
		fn:            fn,
		ready:         make(chan struct{}),
		cleaned:       make(chan struct{}),
		progressState: newProgressState(),
	}
	ctx := newContext(c) // newSharedContext
	pr, pctx, closeProgressWriter := progress.NewContext(context.Background())

	c.progressCtx = pctx
	c.ctx = ctx
	c.closeProgressWriter = closeProgressWriter

	go c.progressState.run(pr) // TODO: remove this, wrap writer instead

	return c
}

func (c *call[T]) run() {
	defer c.closeProgressWriter(errors.WithStack(context.Canceled))
	ctx, cancel := context.WithCancelCause(c.ctx)
	defer func() { cancel(errors.WithStack(context.Canceled)) }()
	v, err := c.fn(ctx)
	c.mu.Lock()
	c.result = v
	c.err = err
	c.mu.Unlock()
	close(c.ready)
}

func (c *call[T]) wait(ctx context.Context) (v T, err error) {
	var empty T
	c.mu.Lock()
	// detect case where caller has just returned, let it clean up before
	select {
	case <-c.ready:
		c.mu.Unlock()
		if c.err != nil { // on error retry
			<-c.cleaned
			return empty, errRetry
		}
		pw, ok, _ := progress.NewFromContext(ctx)
		if ok {
			c.progressState.add(pw)
		}
		return c.result, nil

	case <-c.ctx.done: // could return if no error
		c.mu.Unlock()
		<-c.cleaned
		return empty, errRetry
	default:
	}

	pw, ok, ctx := progress.NewFromContext(ctx)
	if ok {
		c.progressState.add(pw)
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer func() { cancel(errors.WithStack(context.Canceled)) }()

	c.ctxs = append(c.ctxs, ctx)

	c.mu.Unlock()

	go c.once.Do(c.run)

	select {
	case <-ctx.Done():
		if c.ctx.checkDone() {
			// if this cancelled the last context, then wait for function to shut down
			// and don't accept any more callers
			<-c.ready
			return c.result, c.err
		}
		if ok {
			c.progressState.close(pw)
		}
		return empty, context.Cause(ctx)
	case <-c.ready:
		return c.result, c.err // shared not implemented yet
	}
}

func (c *call[T]) Deadline() (deadline time.Time, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ctx := range c.ctxs {
		select {
		case <-ctx.Done():
		default:
			dl, ok := ctx.Deadline()
			if ok {
				return dl, ok
			}
		}
	}
	return time.Time{}, false
}

func (c *call[T]) Done() <-chan struct{} {
	return c.ctx.done
}

func (c *call[T]) Err() error {
	select {
	case <-c.ctx.Done():
		return c.ctx.err
	default:
		return nil
	}
}

func (c *call[T]) Value(key interface{}) interface{} {
	if key == contextKey {
		return c.progressState
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx := c.progressCtx
	select {
	case <-ctx.Done():
	default:
		if v := ctx.Value(key); v != nil {
			return v
		}
	}

	if len(c.ctxs) > 0 {
		ctx = c.ctxs[0]
		select {
		case <-ctx.Done():
		default:
			if v := ctx.Value(key); v != nil {
				return v
			}
		}
	}

	return nil
}

type sharedContext[T any] struct {
	*call[T]
	done chan struct{}
	err  error
}

func newContext[T any](c *call[T]) *sharedContext[T] {
	return &sharedContext[T]{call: c, done: make(chan struct{})}
}

func (sc *sharedContext[T]) checkDone() bool {
	sc.mu.Lock()
	select {
	case <-sc.done:
		sc.mu.Unlock()
		return true
	default:
	}
	var err error
	for _, ctx := range sc.ctxs {
		select {
		case <-ctx.Done():
			// Cause can't be used here because this error is returned for Err() in custom context
			// implementation and unfortunately stdlib does not allow defining Cause() for custom contexts
			err = ctx.Err() //nolint: forbidigo
		default:
			sc.mu.Unlock()
			return false
		}
	}
	sc.err = err
	close(sc.done)
	sc.mu.Unlock()
	return true
}

type rawProgressWriter interface {
	WriteRawProgress(*progress.Progress) error
	Close() error
}

type progressState struct {
	mu      sync.Mutex
	items   map[string]*progress.Progress
	writers []rawProgressWriter
	done    bool
}

func newProgressState() *progressState {
	return &progressState{
		items: make(map[string]*progress.Progress),
	}
}

func (ps *progressState) run(pr progress.Reader) {
	for {
		p, err := pr.Read(context.TODO())
		if err != nil {
			if err == io.EOF {
				ps.mu.Lock()
				ps.done = true
				ps.mu.Unlock()
				for _, w := range ps.writers {
					w.Close()
				}
			}
			return
		}
		ps.mu.Lock()
		for _, p := range p {
			for _, w := range ps.writers {
				w.WriteRawProgress(p)
			}
			ps.items[p.ID] = p
		}
		ps.mu.Unlock()
	}
}

func (ps *progressState) add(pw progress.Writer) {
	rw, ok := pw.(rawProgressWriter)
	if !ok {
		return
	}
	ps.mu.Lock()
	plist := make([]*progress.Progress, 0, len(ps.items))
	for _, p := range ps.items {
		plist = append(plist, p)
	}
	sort.Slice(plist, func(i, j int) bool {
		return plist[i].Timestamp.Before(plist[j].Timestamp)
	})
	for _, p := range plist {
		rw.WriteRawProgress(p)
	}
	if ps.done {
		rw.Close()
	} else {
		ps.writers = append(ps.writers, rw)
	}
	ps.mu.Unlock()
}

func (ps *progressState) close(pw progress.Writer) {
	rw, ok := pw.(rawProgressWriter)
	if !ok {
		return
	}
	ps.mu.Lock()
	for i, w := range ps.writers {
		if w == rw {
			w.Close()
			ps.writers = append(ps.writers[:i], ps.writers[i+1:]...)
			break
		}
	}
	ps.mu.Unlock()
}
