package progress

import (
	"maps"
	"sort"
	"sync"
	"time"
)

type rawProgressWriter interface {
	WriteRawProgress(*Progress) error
	Close() error
}

type MultiWriter struct {
	mu      sync.Mutex
	items   []*Progress
	writers map[rawProgressWriter]struct{}
	meta    map[string]interface{}
}

var _ rawProgressWriter = &MultiWriter{}

func NewMultiWriter(opts ...WriterOption) *MultiWriter {
	mw := &MultiWriter{
		writers: map[rawProgressWriter]struct{}{},
		meta:    map[string]interface{}{},
	}
	for _, o := range opts {
		o(mw)
	}
	return mw
}

func (ps *MultiWriter) Add(pw Writer) {
	rw, ok := pw.(rawProgressWriter)
	if !ok {
		return
	}
	if pws, ok := rw.(*MultiWriter); ok {
		if pws.contains(ps) {
			// this would cause a deadlock, so we should panic instead
			// NOTE: this can be caused by a cycle in the scheduler states,
			// which is created by a series of unfortunate edge merges
			panic("multiwriter loop detected")
		}
	}

	ps.mu.Lock()
	plist := make([]*Progress, 0, len(ps.items))
	plist = append(plist, ps.items...)
	sort.Slice(plist, func(i, j int) bool {
		return plist[i].Timestamp.Before(plist[j].Timestamp)
	})
	for _, p := range plist {
		rw.WriteRawProgress(p)
	}
	ps.writers[rw] = struct{}{}
	ps.mu.Unlock()
}

func (ps *MultiWriter) Delete(pw Writer) {
	rw, ok := pw.(rawProgressWriter)
	if !ok {
		return
	}

	ps.mu.Lock()
	delete(ps.writers, rw)
	ps.mu.Unlock()
}

func (ps *MultiWriter) Write(id string, v interface{}) error {
	p := &Progress{
		ID:        id,
		Timestamp: time.Now(),
		Sys:       v,
		meta:      ps.meta,
	}
	return ps.writeRawProgress(p)
}

func (ps *MultiWriter) WriteRawProgress(p *Progress) error {
	meta := p.meta
	if len(ps.meta) > 0 {
		meta = map[string]interface{}{}
		maps.Copy(meta, p.meta)
		for k, v := range ps.meta {
			if _, ok := meta[k]; !ok {
				meta[k] = v
			}
		}
	}
	p.meta = meta
	return ps.writeRawProgress(p)
}

func (ps *MultiWriter) writeRawProgress(p *Progress) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.items = append(ps.items, p)
	for w := range ps.writers {
		if err := w.WriteRawProgress(p); err != nil {
			return err
		}
	}
	return nil
}

func (ps *MultiWriter) Close() error {
	return nil
}

func (ps *MultiWriter) contains(pw rawProgressWriter) bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	_, ok := ps.writers[pw]
	if ok {
		return true
	}

	for w := range ps.writers {
		w, ok := w.(*MultiWriter)
		if !ok {
			continue
		}
		if w.contains(pw) {
			return true
		}
	}

	return false
}
