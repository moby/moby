package fsutil

import (
	"container/list"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

const rootCacheDefaultSize = 128

type rootCache struct {
	mu      sync.Mutex
	maxSize int
	entries map[string]*rootCacheEntry
	lru     *list.List
	closed  bool
}

type rootCacheEntry struct {
	path     string
	root     Root
	refs     int
	evicted  bool
	elem     *list.Element
	closeErr error
}

type rootLease struct {
	root    Root
	base    string
	entry   *rootCacheEntry
	cache   *rootCache
	release sync.Once
}

func newRootCache(root Root, maxSize int) *rootCache {
	if maxSize <= 0 {
		maxSize = rootCacheDefaultSize
	}
	return &rootCache{
		maxSize: maxSize,
		entries: map[string]*rootCacheEntry{
			".": {path: ".", root: root},
		},
		lru: list.New(),
	}
}

func (c *rootCache) get(path string) (*rootLease, error) {
	path = cleanRootPath(path)
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, errors.WithStack(os.ErrClosed)
	}

	entry, err := c.getDirLocked(dir)
	if err != nil {
		return nil, err
	}
	entry.refs++

	return &rootLease{
		root:  entry.root,
		base:  base,
		entry: entry,
		cache: c,
	}, nil
}

func (c *rootCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true
	var err error
	for path, entry := range c.entries {
		if path == "." {
			continue
		}
		delete(c.entries, path)
		if entry.elem != nil {
			c.lru.Remove(entry.elem)
			entry.elem = nil
		}
		entry.evicted = true
		if entry.refs == 0 {
			if err2 := closeRootCacheEntry(entry); err == nil {
				err = err2
			}
		}
	}
	return err
}

func (l *rootLease) Release() error {
	var err error
	l.release.Do(func() {
		l.cache.mu.Lock()
		defer l.cache.mu.Unlock()

		l.entry.refs--
		if l.entry.refs == 0 && l.entry.evicted {
			err = closeRootCacheEntry(l.entry)
		}
	})
	return err
}

func (c *rootCache) getDirLocked(dir string) (*rootCacheEntry, error) {
	entry := c.entries["."]
	if dir == "." {
		return entry, nil
	}

	path := "."
	for _, component := range rootCacheComponents(dir) {
		nextPath := component
		if path != "." {
			nextPath = filepath.Join(path, component)
		}

		nextEntry, ok := c.entries[nextPath]
		if ok && !nextEntry.evicted {
			c.touchLocked(nextEntry)
			entry = nextEntry
			path = nextPath
			continue
		}

		osroot, err := entry.root.OpenRoot(component)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		nextEntry = &rootCacheEntry{
			path: nextPath,
			root: NewRoot(osroot),
		}
		nextEntry.elem = c.lru.PushFront(nextEntry)
		c.entries[nextPath] = nextEntry
		entry = nextEntry
		path = nextPath
		c.evictLocked()
	}

	return entry, nil
}

func (c *rootCache) touchLocked(entry *rootCacheEntry) {
	if entry.elem != nil {
		c.lru.MoveToFront(entry.elem)
	}
}

func (c *rootCache) evictLocked() {
	for len(c.entries)-1 > c.maxSize {
		elem := c.lru.Back()
		if elem == nil {
			return
		}
		entry := elem.Value.(*rootCacheEntry)
		c.lru.Remove(elem)
		entry.elem = nil
		entry.evicted = true
		delete(c.entries, entry.path)
		if entry.refs == 0 {
			entry.closeErr = closeRootCacheEntry(entry)
		}
	}
}

func closeRootCacheEntry(entry *rootCacheEntry) error {
	if entry.closeErr != nil {
		return entry.closeErr
	}
	if err := entry.root.Close(); err != nil {
		entry.closeErr = errors.WithStack(err)
	}
	return entry.closeErr
}

func rootCacheComponents(dir string) []string {
	if dir == "." {
		return nil
	}
	return strings.Split(dir, string(filepath.Separator))
}
