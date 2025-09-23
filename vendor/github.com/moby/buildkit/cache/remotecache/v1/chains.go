package cacheimport

import (
	"context"
	"encoding/binary"
	"maps"
	"slices"
	"strings"
	"unique"

	"github.com/cespare/xxhash/v2"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func NewCacheChains() *CacheChains {
	return &CacheChains{roots: map[digest.Digest]*item{}}
}

type CacheChains struct {
	roots map[digest.Digest]*item
}

var _ solver.CacheExporterTarget = &CacheChains{}

func (c *CacheChains) computeIDs() {
	for it := range c.leaves() {
		it.computeID()
	}
}

func (c *CacheChains) leaves() map[*item]struct{} {
	leafs := map[*item]struct{}{}
	visited := map[*item]struct{}{}
	for _, it := range c.roots {
		it.walkChildren(func(i *item) error {
			if len(i.children) == 0 {
				leafs[i] = struct{}{}
			}
			return nil
		}, visited)
	}
	return leafs
}

func (c *CacheChains) Add(dgst digest.Digest, deps [][]solver.CacheLink, results []solver.CacheExportResult) (solver.CacheExporterRecord, bool, error) {
	if strings.HasPrefix(dgst.String(), "random:") {
		return nil, false, nil
	}
	r := &item{
		dgst:    dgst,
		results: results,
		cc:      c,
	}

	if len(deps) == 0 {
		if prev, ok := c.roots[dgst]; ok {
			r = prev
		}
		for _, rr := range results {
			r.addResult(rr)
		}
		c.roots[dgst] = r
		return r, true, nil
	}

	matchDeps := make([]func() map[*item]struct{}, len(deps))
	for i, dd := range deps {
		if len(dd) == 0 {
			return nil, false, errors.Errorf("empty dependency for %s", dgst)
		}
		type itemWithSelector struct {
			Src      *item
			Selector string
		}
		items := make([]itemWithSelector, len(dd))
		for ii, d := range dd {
			it, ok := d.Src.(*item)
			if !ok {
				return nil, false, errors.Errorf("invalid dependency type %T for %s", d.Src, dgst)
			}
			if it.cc != c {
				return nil, false, errors.Errorf("dependency %s is not part of the same cache chain", it.dgst)
			}
			items[ii] = itemWithSelector{
				Src:      it,
				Selector: d.Selector,
			}
		}
		matchDeps[i] = func() map[*item]struct{} {
			candidates := map[*item]struct{}{}
			for _, it := range items {
				maps.Copy(candidates, it.Src.children[unique.Make(linkv2{
					selector: it.Selector,
					index:    i,
					digest:   dgst,
				})])
			}
			return candidates
		}
	}
	items := IntersectAll(matchDeps)

	if len(items) > 1 {
		var main *item
		for it := range items {
			main = it
			break
		}
		for it := range items {
			if it == main {
				continue
			}
			for l, m := range it.children {
				if main.children == nil {
					main.children = map[unique.Handle[linkv2]]map[*item]struct{}{}
				}
				if _, ok := main.children[l]; !ok {
					main.children[l] = map[*item]struct{}{}
				}
				for ch := range m {
					main.children[l][ch] = struct{}{}
					for i, links := range ch.parents {
						newlinks := map[link]struct{}{}
						for l := range links {
							if l.src == it {
								l.src = main
							}
							newlinks[l] = struct{}{}
						}
						ch.parents[i] = newlinks
					}
				}
			}
			for _, rr := range it.results {
				main.addResult(rr)
			}
		}
		items = map[*item]struct{}{main: {}}
	}

	for it := range items {
		r = it
		for _, rr := range results {
			r.addResult(rr)
		}

		// make sure that none of the deps are children of r
		allChildren := map[*item]struct{}{}
		if err := r.walkChildren(func(i *item) error {
			allChildren[i] = struct{}{}
			return nil
		}, map[*item]struct{}{}); err != nil {
			return nil, false, errors.Wrapf(err, "failed to walk children of %s", dgst)
		}
		for i, dd := range deps {
			for j, d := range dd {
				if _, ok := allChildren[d.Src.(*item)]; ok {
					deps[i][j].Src = nil
				}
			}
		}
		break
	}
	for i, dd := range deps {
		for _, d := range dd {
			if d.Src == nil {
				continue
			}
			d.Src.(*item).addChild(r, i, d.Selector)
		}
	}
	return r, true, nil
}

func IntersectAll[T comparable](
	funcs []func() map[T]struct{},
) map[T]struct{} {
	if len(funcs) == 0 {
		return nil
	}

	intersection := funcs[0]()

	for _, f := range funcs[1:] {
		next := f()
		for k := range intersection {
			if _, ok := next[k]; !ok {
				delete(intersection, k)
			}
		}
		if len(intersection) == 0 {
			return nil
		}
	}

	return intersection
}

// Marshal converts the cache chains structure into a cache config and a
// collection of providers for reading the results from.
//
// Marshal aims to validate, normalize and sort the output to ensure a
// consistent digest (since cache configs are typically uploaded and stored in
// content-addressable OCI registries).
func (c *CacheChains) Marshal(ctx context.Context) (*CacheConfig, DescriptorProvider, error) {
	st := &marshalState{
		chainsByID:    map[string]int{},
		descriptors:   DescriptorProvider{},
		recordsByItem: map[*item]int{},
	}

	for it := range c.leaves() {
		if err := marshalItem(ctx, it, st); err != nil {
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
	Descriptor   ocispecs.Descriptor
	Provider     content.Provider
	InfoProvider content.InfoProvider
}

func (p DescriptorProviderPair) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	return p.Provider.ReaderAt(ctx, desc)
}

func (p DescriptorProviderPair) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	if p.InfoProvider != nil {
		return p.InfoProvider.Info(ctx, dgst)
	}
	if dgst != p.Descriptor.Digest {
		return content.Info{}, errors.Errorf("content not found %s", dgst)
	}
	return content.Info{
		Digest: p.Descriptor.Digest,
		Size:   p.Descriptor.Size,
	}, nil
}

func (p DescriptorProviderPair) UnlazySession(desc ocispecs.Descriptor) session.Group {
	type unlazySession interface {
		UnlazySession(ocispecs.Descriptor) session.Group
	}
	if cd, ok := p.Provider.(unlazySession); ok {
		return cd.UnlazySession(desc)
	}
	return nil
}

func (p DescriptorProviderPair) SnapshotLabels(descs []ocispecs.Descriptor, index int) map[string]string {
	type snapshotLabels interface {
		SnapshotLabels([]ocispecs.Descriptor, int) map[string]string
	}
	if cd, ok := p.Provider.(snapshotLabels); ok {
		return cd.SnapshotLabels(descs, index)
	}
	return nil
}

type linkv2 struct {
	selector string
	index    int
	digest   digest.Digest
}

// item is an implementation of a record in the cache chain. After validation,
// normalization and marshalling into the cache config, the item results form
// into the "layers", while the digests and the links form into the "records".
type item struct {
	solver.CacheExporterRecordBase

	id string

	// dgst is the unique identifier for each record.
	// This *roughly* corresponds to an edge (vertex cachekey + index) in the
	// solver - however, a single vertex can produce multiple unique cache keys
	// (e.g. fast/slow), so it's a one-to-many relation.
	dgst digest.Digest

	children map[unique.Handle[linkv2]]map[*item]struct{}

	parents []map[link]struct{}

	results []solver.CacheExportResult

	cc *CacheChains
}

// link is a pointer to an item, with an optional selector.
type link struct {
	src      *item
	selector string
}

func (c *item) addChild(src *item, index int, selector string) {
	if c.children == nil {
		c.children = map[unique.Handle[linkv2]]map[*item]struct{}{}
	}
	h := unique.Make(linkv2{
		selector: selector,
		index:    index,
		digest:   src.dgst,
	})
	m, ok := c.children[h]
	if !ok {
		m = map[*item]struct{}{}
		c.children[h] = m
	}
	if _, ok := m[src]; ok {
		return
	}
	m[src] = struct{}{}

	for index >= len(src.parents) {
		src.parents = append(src.parents, map[link]struct{}{})
	}
	src.parents[index][link{src: c, selector: selector}] = struct{}{}
}

func (c *item) addResult(r solver.CacheExportResult) {
	var exists bool
	for _, rr := range c.results {
		if !rr.CreatedAt.Equal(r.CreatedAt) {
			continue
		}
		if len(rr.Result.Descriptors) != len(r.Result.Descriptors) {
			continue
		}
		for i, d := range rr.Result.Descriptors {
			if d.Digest != r.Result.Descriptors[i].Digest {
				continue
			}
		}
		exists = true
		break
	}
	if !exists {
		c.results = append(c.results, r)
	}
}

func (c *item) computeID() {
	if c.id != "" {
		return
	}

	if len(c.parents) == 0 {
		c.id = c.dgst.String()
		return
	}

	// deterministic ID
	h := xxhash.New()
	h.Write([]byte(c.dgst.String()))
	h.Write([]byte{0})

	for idx, m := range c.parents {
		binary.Write(h, binary.LittleEndian, uint32(idx))
		h.Write([]byte{0})
		for l := range m {
			if l.src.id == "" {
				l.src.computeID()
			}
			h.Write([]byte(l.src.id))
			h.Write([]byte{0})
			h.Write([]byte(l.selector))
			h.Write([]byte{0})
		}
	}
	c.id = string(h.Sum(nil))
}

func (c *item) bestResult() *solver.CacheExportResult {
	if len(c.results) == 0 {
		return nil
	}
	slices.SortFunc(c.results, func(a, b solver.CacheExportResult) int {
		return b.CreatedAt.Compare(a.CreatedAt)
	})
	return &c.results[0]
}

func (c *item) walkChildren(fn func(i *item) error, visited map[*item]struct{}) error {
	if _, ok := visited[c]; ok {
		return nil
	}
	visited[c] = struct{}{}
	if err := fn(c); err != nil {
		return err
	}
	for _, ch := range c.children {
		for it := range ch {
			if err := it.walkChildren(fn, visited); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *item) walkAllResults(fn func(i *item) error, visited map[*item]struct{}) error {
	if _, ok := visited[c]; ok {
		return nil
	}
	visited[c] = struct{}{}
	if err := fn(c); err != nil {
		return err
	}
	for _, links := range c.parents {
		for l := range links {
			if err := l.src.walkAllResults(fn, visited); err != nil {
				return err
			}
		}
	}
	return nil
}
