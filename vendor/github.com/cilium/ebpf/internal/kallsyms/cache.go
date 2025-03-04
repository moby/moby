package kallsyms

import "sync"

type cache[K, V comparable] struct {
	m sync.Map
}

func (c *cache[K, V]) Load(key K) (value V, _ bool) {
	v, ok := c.m.Load(key)
	if !ok {
		return value, false
	}
	value = v.(V)
	return value, true
}

func (c *cache[K, V]) Store(key K, value V) {
	c.m.Store(key, value)
}
