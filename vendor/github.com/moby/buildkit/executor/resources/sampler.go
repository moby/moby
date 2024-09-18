package resources

import (
	"sync"
	"time"
)

type WithTimestamp interface {
	Timestamp() time.Time
}

type Sampler[T WithTimestamp] struct {
	mu          sync.Mutex
	minInterval time.Duration
	maxSamples  int
	callback    func(ts time.Time) (T, error)
	doneOnce    sync.Once
	done        chan struct{}
	running     bool
	subs        map[*Sub[T]]struct{}
}

type Sub[T WithTimestamp] struct {
	sampler  *Sampler[T]
	interval time.Duration
	first    time.Time
	last     time.Time
	samples  []T
	err      error
}

func (s *Sub[T]) Close(captureLast bool) ([]T, error) {
	s.sampler.mu.Lock()
	delete(s.sampler.subs, s)

	if s.err != nil {
		s.sampler.mu.Unlock()
		return nil, s.err
	}
	current := s.first
	out := make([]T, 0, len(s.samples)+1)
	for i, v := range s.samples {
		ts := v.Timestamp()
		if i == 0 || ts.Sub(current) >= s.interval {
			out = append(out, v)
			current = ts
		}
	}
	s.sampler.mu.Unlock()

	if captureLast {
		v, err := s.sampler.callback(time.Now())
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}

	return out, nil
}

func NewSampler[T WithTimestamp](minInterval time.Duration, maxSamples int, cb func(time.Time) (T, error)) *Sampler[T] {
	s := &Sampler[T]{
		minInterval: minInterval,
		maxSamples:  maxSamples,
		callback:    cb,
		done:        make(chan struct{}),
		subs:        make(map[*Sub[T]]struct{}),
	}
	return s
}

func (s *Sampler[T]) Record() *Sub[T] {
	ss := &Sub[T]{
		interval: s.minInterval,
		first:    time.Now(),
		sampler:  s,
	}
	s.mu.Lock()
	s.subs[ss] = struct{}{}
	if !s.running {
		s.running = true
		go s.run()
	}
	s.mu.Unlock()
	return ss
}

func (s *Sampler[T]) run() {
	ticker := time.NewTimer(s.minInterval)
	for {
		select {
		case <-s.done:
			ticker.Stop()
			return
		case <-ticker.C:
			tm := time.Now()
			s.mu.Lock()
			active := make([]*Sub[T], 0, len(s.subs))
			for ss := range s.subs {
				if tm.Sub(ss.last) < ss.interval {
					continue
				}
				ss.last = tm
				active = append(active, ss)
			}
			s.mu.Unlock()
			ticker = time.NewTimer(s.minInterval)
			if len(active) == 0 {
				continue
			}
			value, err := s.callback(tm)
			s.mu.Lock()
			for _, ss := range active {
				if _, found := s.subs[ss]; !found {
					continue // skip if Close() was called while the lock was released
				}
				if err != nil {
					ss.err = err
				} else {
					ss.samples = append(ss.samples, value)
					ss.err = nil
				}
				dur := ss.last.Sub(ss.first)
				if time.Duration(ss.interval)*time.Duration(s.maxSamples) <= dur {
					ss.interval *= 2
				}
			}
			s.mu.Unlock()
		}
	}
}

func (s *Sampler[T]) Close() error {
	s.doneOnce.Do(func() {
		close(s.done)
	})
	return nil
}
