package cacheimport

import (
	"strings"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/moby/buildkit/solver"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func NewCacheChains() *CacheChains {
	return &CacheChains{visited: map[interface{}]struct{}{}}
}

type CacheChains struct {
	items   []*item
	visited map[interface{}]struct{}
}

func (c *CacheChains) Add(dgst digest.Digest) solver.CacheExporterRecord {
	if strings.HasPrefix(dgst.String(), "random:") {
		return &nopRecord{}
	}
	it := &item{c: c, dgst: dgst, backlinks: map[*item]struct{}{}}
	c.items = append(c.items, it)
	return it
}

func (c *CacheChains) Visit(v interface{}) {
	c.visited[v] = struct{}{}
}

func (c *CacheChains) Visited(v interface{}) bool {
	_, ok := c.visited[v]
	return ok
}

func (c *CacheChains) normalize() error {
	st := &normalizeState{
		added: map[*item]*item{},
		links: map[*item]map[nlink]map[digest.Digest]struct{}{},
		byKey: map[digest.Digest]*item{},
	}

	validated := make([]*item, 0, len(c.items))
	for _, it := range c.items {
		it.validate()
	}
	for _, it := range c.items {
		if !it.invalid {
			validated = append(validated, it)
		}
	}
	c.items = validated

	for _, it := range c.items {
		_, err := normalizeItem(it, st)
		if err != nil {
			return err
		}
	}

	items := make([]*item, 0, len(st.byKey))
	for _, it := range st.byKey {
		items = append(items, it)
	}
	c.items = items
	return nil
}

func (c *CacheChains) Marshal() (*CacheConfig, DescriptorProvider, error) {
	if err := c.normalize(); err != nil {
		return nil, nil, err
	}

	st := &marshalState{
		chainsByID:    map[string]int{},
		descriptors:   DescriptorProvider{},
		recordsByItem: map[*item]int{},
	}

	for _, it := range c.items {
		if err := marshalItem(it, st); err != nil {
			return nil, nil, err
		}
	}

	cc := CacheConfig{
		Layers:  st.layers,
		Records: st.records,
	}
	sortConfig(&cc)

	return &cc, st.descriptors, nil
}

type DescriptorProvider map[digest.Digest]DescriptorProviderPair

type DescriptorProviderPair struct {
	Descriptor ocispec.Descriptor
	Provider   content.Provider
}

type item struct {
	c    *CacheChains
	dgst digest.Digest

	result     *solver.Remote
	resultTime time.Time

	links     []map[link]struct{}
	backlinks map[*item]struct{}
	invalid   bool
}

type link struct {
	src      *item
	selector string
}

func (c *item) AddResult(createdAt time.Time, result *solver.Remote) {
	c.resultTime = createdAt
	c.result = result
}

func (c *item) LinkFrom(rec solver.CacheExporterRecord, index int, selector string) {
	src, ok := rec.(*item)
	if !ok {
		return
	}

	for {
		if index < len(c.links) {
			break
		}
		c.links = append(c.links, map[link]struct{}{})
	}

	c.links[index][link{src: src, selector: selector}] = struct{}{}
	src.backlinks[c] = struct{}{}
}

func (c *item) validate() {
	for _, m := range c.links {
		if len(m) == 0 {
			c.invalid = true
			for bl := range c.backlinks {
				changed := false
				for _, m := range bl.links {
					for l := range m {
						if l.src == c {
							delete(m, l)
							changed = true
						}
					}
				}
				if changed {
					bl.validate()
				}
			}
			return
		}
	}
}

func (c *item) walkAllResults(fn func(i *item) error, visited map[*item]struct{}) error {
	if _, ok := visited[c]; ok {
		return nil
	}
	visited[c] = struct{}{}
	if err := fn(c); err != nil {
		return err
	}
	for _, links := range c.links {
		for l := range links {
			if err := l.src.walkAllResults(fn, visited); err != nil {
				return err
			}
		}
	}
	return nil
}

type nopRecord struct {
}

func (c *nopRecord) AddResult(createdAt time.Time, result *solver.Remote) {
}

func (c *nopRecord) LinkFrom(rec solver.CacheExporterRecord, index int, selector string) {
}

var _ solver.CacheExporterTarget = &CacheChains{}
