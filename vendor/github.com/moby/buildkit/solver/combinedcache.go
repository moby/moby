package solver

import (
	"context"
	"strings"
	"sync"

	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func newCombinedCacheManager(cms []CacheManager, main CacheManager) CacheManager {
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

func (cm *combinedCacheManager) Load(ctx context.Context, rec *CacheRecord) (Result, error) {
	res, err := rec.cacheManager.Load(ctx, rec)
	if err != nil {
		return nil, err
	}
	if _, err := cm.main.Save(rec.key, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (cm *combinedCacheManager) Save(key *CacheKey, s Result) (*ExportableCacheKey, error) {
	return cm.main.Save(key, s)
}

func (cm *combinedCacheManager) Records(ck *CacheKey) ([]*CacheRecord, error) {
	if len(ck.ids) == 0 {
		return nil, errors.Errorf("no results")
	}

	records := map[string]*CacheRecord{}
	var mu sync.Mutex

	eg, _ := errgroup.WithContext(context.TODO())
	for c := range ck.ids {
		func(c *cacheManager) {
			eg.Go(func() error {
				recs, err := c.Records(ck)
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
		}(c)
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
