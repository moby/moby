package solver

import (
	"context"
	"strings"
	"sync"
	"time"

	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func NewCombinedCacheManager(cms []CacheManager, main CacheManager) CacheManager {
	return &combinedCacheManager{cms: cms, main: main}
}

type combinedCacheManager struct {
	cms    []CacheManager
	main   CacheManager
	id     string
	idOnce sync.Once
}

func (cm *combinedCacheManager) ID() string {
	cm.idOnce.Do(func() {
		ids := make([]string, len(cm.cms))
		for i, c := range cm.cms {
			ids[i] = c.ID()
		}
		cm.id = digest.FromBytes([]byte(strings.Join(ids, ","))).String()
	})
	return cm.id
}

func (cm *combinedCacheManager) Query(inp []CacheKeyWithSelector, inputIndex Index, dgst digest.Digest, outputIndex Index) ([]*CacheKey, error) {
	eg, _ := errgroup.WithContext(context.TODO())
	keys := make(map[string]*CacheKey, len(cm.cms))
	var mu sync.Mutex
	for _, c := range cm.cms {
		func(c CacheManager) {
			eg.Go(func() error {
				recs, err := c.Query(inp, inputIndex, dgst, outputIndex)
				if err != nil {
					return err
				}
				mu.Lock()
				for _, r := range recs {
					if _, ok := keys[r.ID]; !ok || c == cm.main {
						keys[r.ID] = r
					}
				}
				mu.Unlock()
				return nil
			})
		}(c)
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	out := make([]*CacheKey, 0, len(keys))
	for _, k := range keys {
		out = append(out, k)
	}
	return out, nil
}

func (cm *combinedCacheManager) Load(ctx context.Context, rec *CacheRecord) (res Result, err error) {
	results, err := rec.cacheManager.LoadWithParents(ctx, rec)
	if err != nil {
		return nil, err
	}
	defer func() {
		for i, res := range results {
			if err == nil && i == 0 {
				continue
			}
			res.Result.Release(context.TODO())
		}
	}()
	if rec.cacheManager != cm.main && cm.main != nil {
		for _, res := range results {
			if _, err := cm.main.Save(res.CacheKey, res.Result, res.CacheResult.CreatedAt); err != nil {
				return nil, err
			}
		}
	}
	if len(results) == 0 { // TODO: handle gracefully
		return nil, errors.Errorf("failed to load deleted cache")
	}
	return results[0].Result, nil
}

func (cm *combinedCacheManager) Save(key *CacheKey, s Result, createdAt time.Time) (*ExportableCacheKey, error) {
	if cm.main == nil {
		return nil, nil
	}
	return cm.main.Save(key, s, createdAt)
}

func (cm *combinedCacheManager) Records(ctx context.Context, ck *CacheKey) ([]*CacheRecord, error) {
	ck.mu.RLock()
	if len(ck.ids) == 0 {
		ck.mu.RUnlock()
		return nil, errors.Errorf("no results")
	}

	cms := make([]*cacheManager, 0, len(ck.ids))
	for cm := range ck.ids {
		cms = append(cms, cm)
	}
	ck.mu.RUnlock()

	records := map[string]*CacheRecord{}
	var mu sync.Mutex

	eg, _ := errgroup.WithContext(context.TODO())
	for _, c := range cms {
		c := c
		eg.Go(func() error {
			recs, err := c.Records(ctx, ck)
			if err != nil {
				return err
			}
			mu.Lock()
			for _, rec := range recs {
				if _, ok := records[rec.ID]; !ok || c == cm.main {
					if c == cm.main {
						rec.Priority = 1
					}
					records[rec.ID] = rec
				}
			}
			mu.Unlock()
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	out := make([]*CacheRecord, 0, len(records))
	for _, rec := range records {
		out = append(out, rec)
	}
	return out, nil
}
