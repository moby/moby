package cacheimport

import (
	"context"
	"slices"
	"time"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

func NewCacheKeyStorage(cc *CacheChains, w worker.Worker) (solver.CacheKeyStorage, solver.CacheResultStorage, error) {
	storage := &cacheKeyStorage{
		byID:     map[string]*itemWithOutgoingLinks{},
		byItem:   map[*item]string{},
		byResult: map[string]map[string]struct{}{},
	}

	cc.computeIDs()

	for it := range cc.leaves() {
		visited := make(map[*item]*itemWithOutgoingLinks)
		if _, err := addItemToStorage(storage, it, visited); err != nil {
			return nil, nil, err
		}
	}

	results := &cacheResultStorage{
		w:        w,
		byID:     storage.byID,
		byItem:   storage.byItem,
		byResult: storage.byResult,
	}

	return storage, results, nil
}

func addItemToStorage(k *cacheKeyStorage, it *item, visited map[*item]*itemWithOutgoingLinks) (*itemWithOutgoingLinks, error) {
	if v, ok := visited[it]; ok {
		return v, nil
	}
	visited[it] = nil

	if id, ok := k.byItem[it]; ok {
		if id == "" {
			return nil, errors.Errorf("invalid loop")
		}
		return k.byID[id], nil
	}

	id := it.id
	k.byItem[it] = ""

	for i, m := range it.parents {
		for l := range m {
			src, err := addItemToStorage(k, l.src, visited)
			if err != nil {
				return nil, err
			}
			if src == nil {
				continue
			}
			cl := nlink{
				input:    i,
				dgst:     it.dgst,
				selector: l.selector,
			}
			src.links[cl] = append(src.links[cl], id)
		}
	}

	k.byItem[it] = id

	itl := &itemWithOutgoingLinks{
		item:  it,
		links: map[nlink][]string{},
	}

	k.byID[id] = itl

	seen := map[string]struct{}{}
	for _, res := range it.results {
		resultID := remoteID(res.Result)
		if _, ok := seen[resultID]; ok {
			continue
		}
		seen[resultID] = struct{}{}
		ids, ok := k.byResult[resultID]
		if !ok {
			ids = map[string]struct{}{}
			k.byResult[resultID] = ids
		}
		ids[id] = struct{}{}
	}
	visited[it] = itl
	return itl, nil
}

type cacheKeyStorage struct {
	byID     map[string]*itemWithOutgoingLinks
	byItem   map[*item]string
	byResult map[string]map[string]struct{}
}

type itemWithOutgoingLinks struct {
	*item
	links map[nlink][]string
}

func (cs *cacheKeyStorage) Exists(id string) bool {
	_, ok := cs.byID[id]
	return ok
}

func (cs *cacheKeyStorage) Walk(cb func(id string) error) error {
	for id := range cs.byID {
		if err := cb(id); err != nil {
			return err
		}
	}
	return nil
}

func (cs *cacheKeyStorage) WalkResults(id string, fn func(solver.CacheResult) error) error {
	it, ok := cs.byID[id]
	if !ok {
		return nil
	}
	seen := map[string]struct{}{}
	for _, res := range it.results {
		id := remoteID(res.Result)
		if _, ok := seen[id]; ok {
			continue
		}
		if err := fn(solver.CacheResult{ID: id, CreatedAt: res.CreatedAt}); err != nil {
			return err
		}
		seen[id] = struct{}{}
	}
	return nil
}

func (cs *cacheKeyStorage) Load(id string, resultID string) (solver.CacheResult, error) {
	var res solver.CacheResult
	if err := cs.WalkResults(id, func(r solver.CacheResult) error {
		if r.ID == resultID {
			res = r
			return nil
		}
		return nil
	}); err != nil {
		return solver.CacheResult{}, errors.Wrapf(err, "failed to load cache result for %s", id)
	}
	return res, nil
}

func (cs *cacheKeyStorage) AddResult(id string, res solver.CacheResult) error {
	return nil
}

func (cs *cacheKeyStorage) Release(resultID string) error {
	return nil
}
func (cs *cacheKeyStorage) AddLink(id string, link solver.CacheInfoLink, target string) error {
	return nil
}

func (cs *cacheKeyStorage) WalkLinksAll(id string, fn func(id string, link solver.CacheInfoLink) error) error {
	it, ok := cs.byID[id]
	if !ok {
		return nil
	}
	for nl, ids := range it.links {
		for _, id2 := range ids {
			if err := fn(id2, solver.CacheInfoLink{
				Input:    solver.Index(nl.input),
				Selector: digest.Digest(nl.selector),
				Digest:   nl.dgst,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cs *cacheKeyStorage) WalkLinks(id string, link solver.CacheInfoLink, fn func(id string) error) error {
	it, ok := cs.byID[id]
	if !ok {
		return nil
	}
	for _, id := range it.links[nlink{
		dgst:     outputKey(link.Digest, int(link.Output)),
		input:    int(link.Input),
		selector: link.Selector.String(),
	}] {
		if err := fn(id); err != nil {
			return err
		}
	}
	return nil
}

func (cs *cacheKeyStorage) WalkBacklinks(id string, fn func(id string, link solver.CacheInfoLink) error) error {
	for k, it := range cs.byID {
		for nl, ids := range it.links {
			for _, id2 := range ids {
				if id == id2 {
					if err := fn(k, solver.CacheInfoLink{
						Input:    solver.Index(nl.input),
						Selector: digest.Digest(nl.selector),
						Digest:   nl.dgst,
					}); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (cs *cacheKeyStorage) WalkIDsByResult(id string, fn func(id string) error) error {
	ids := cs.byResult[id]
	for id := range ids {
		if err := fn(id); err != nil {
			return err
		}
	}
	return nil
}

func (cs *cacheKeyStorage) HasLink(id string, link solver.CacheInfoLink, target string) bool {
	l := nlink{
		dgst:     outputKey(link.Digest, int(link.Output)),
		input:    int(link.Input),
		selector: link.Selector.String(),
	}
	if it, ok := cs.byID[id]; ok {
		if slices.Contains(it.links[l], target) {
			return true
		}
	}
	return false
}

type cacheResultStorage struct {
	w        worker.Worker
	byID     map[string]*itemWithOutgoingLinks
	byResult map[string]map[string]struct{}
	byItem   map[*item]string
}

func (cs *cacheResultStorage) Save(res solver.Result, createdAt time.Time) (solver.CacheResult, error) {
	return solver.CacheResult{}, errors.Errorf("importer is immutable")
}

func (cs *cacheResultStorage) LoadWithParents(ctx context.Context, res solver.CacheResult) (map[string]solver.Result, error) {
	m := map[string]solver.Result{}

	visited := make(map[*item]struct{})

	ids, ok := cs.byResult[res.ID]
	if !ok || len(ids) == 0 {
		return nil, errors.WithStack(solver.ErrNotFound)
	}

	for id := range ids {
		v, ok := cs.byID[id]
		if ok {
			if _, ok := visited[v.item]; ok {
				continue
			}
			for _, result := range v.results {
				resultID := remoteID(result.Result)
				if resultID == res.ID {
					if err := v.walkAllResults(func(i *item) error {
						for _, subRes := range i.results {
							id, ok := cs.byItem[i]
							if !ok {
								return nil
							}
							if isSubRemote(*subRes.Result, *result.Result) {
								ref, err := cs.w.FromRemote(ctx, subRes.Result)
								if err != nil {
									return err
								}
								m[id] = worker.NewWorkerRefResult(ref, cs.w)
							}
						}
						return nil
					}, visited); err != nil {
						for _, v := range m {
							v.Release(context.TODO())
						}
						return nil, err
					}
				}
			}
		}
	}

	return m, nil
}

func (cs *cacheResultStorage) Load(ctx context.Context, res solver.CacheResult) (solver.Result, error) {
	item := cs.byResultID(res.ID)
	for _, r := range item.results {
		resultID := remoteID(r.Result)
		if resultID != res.ID {
			continue
		}
		ref, err := cs.w.FromRemote(ctx, r.Result)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load result from remote")
		}
		return worker.NewWorkerRefResult(ref, cs.w), nil
	}
	return nil, errors.WithStack(solver.ErrNotFound)
}

func (cs *cacheResultStorage) LoadRemotes(ctx context.Context, res solver.CacheResult, compressionopts *compression.Config, _ session.Group) ([]*solver.Remote, error) {
	if it := cs.byResultID(res.ID); it != nil {
		for _, r := range it.results {
			if compressionopts == nil {
				resultID := remoteID(r.Result)
				if resultID != res.ID {
					continue
				}
				return []*solver.Remote{r.Result}, nil
			}
			// Any of blobs in the remote must meet the specified compression option.
			match := false
			for _, desc := range r.Result.Descriptors {
				m := compression.IsMediaType(compressionopts.Type, desc.MediaType)
				match = match || m
				if compressionopts.Force && !m {
					match = false
					break
				}
			}
			if match {
				return []*solver.Remote{r.Result}, nil
			}
		}
		return nil, nil // return nil as it's best effort.
	}
	return nil, errors.WithStack(solver.ErrNotFound)
}

func (cs *cacheResultStorage) Exists(ctx context.Context, id string) bool {
	return cs.byResultID(id) != nil
}

func (cs *cacheResultStorage) byResultID(resultID string) *itemWithOutgoingLinks {
	m, ok := cs.byResult[resultID]
	if !ok || len(m) == 0 {
		return nil
	}

	for id := range m {
		it, ok := cs.byID[id]
		if ok {
			return it
		}
	}

	return nil
}

// unique ID per remote. this ID is not stable.
func remoteID(r *solver.Remote) string {
	dgstr := digest.Canonical.Digester()
	for _, desc := range r.Descriptors {
		dgstr.Hash().Write([]byte(desc.Digest))
	}
	return dgstr.Digest().String()
}
