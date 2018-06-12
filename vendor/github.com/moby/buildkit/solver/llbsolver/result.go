package llbsolver

import (
	"bytes"
	"context"
	"path"
	"strings"
	"time"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/contenthash"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func NewContentHashFunc(selectors []string) solver.ResultBasedCacheFunc {
	return func(ctx context.Context, res solver.Result) (digest.Digest, error) {
		ref, ok := res.Sys().(*worker.WorkerRef)
		if !ok {
			return "", errors.Errorf("invalid reference: %T", res)
		}

		if len(selectors) == 0 {
			selectors = []string{""}
		}

		dgsts := make([][]byte, len(selectors))

		eg, ctx := errgroup.WithContext(ctx)

		for i, sel := range selectors {
			// FIXME(tonistiigi): enabling this parallelization seems to create wrong results for some big inputs(like gobuild)
			// func(i int) {
			// 	eg.Go(func() error {
			dgst, err := contenthash.Checksum(ctx, ref.ImmutableRef, path.Join("/", sel))
			if err != nil {
				return "", err
			}
			dgsts[i] = []byte(dgst)
			// return nil
			// })
			// }(i)
		}

		if err := eg.Wait(); err != nil {
			return "", err
		}

		return digest.FromBytes(bytes.Join(dgsts, []byte{0})), nil
	}
}

func newCacheResultStorage(wc *worker.Controller) solver.CacheResultStorage {
	return &cacheResultStorage{
		wc: wc,
	}
}

type cacheResultStorage struct {
	wc *worker.Controller
}

func (s *cacheResultStorage) Save(res solver.Result) (solver.CacheResult, error) {
	ref, ok := res.Sys().(*worker.WorkerRef)
	if !ok {
		return solver.CacheResult{}, errors.Errorf("invalid result: %T", res.Sys())
	}
	if ref.ImmutableRef != nil {
		if !cache.HasCachePolicyRetain(ref.ImmutableRef) {
			if err := cache.CachePolicyRetain(ref.ImmutableRef); err != nil {
				return solver.CacheResult{}, err
			}
			ref.ImmutableRef.Metadata().Commit()
		}
	}
	return solver.CacheResult{ID: ref.ID(), CreatedAt: time.Now()}, nil
}
func (s *cacheResultStorage) Load(ctx context.Context, res solver.CacheResult) (solver.Result, error) {
	return s.load(res.ID)
}

func (s *cacheResultStorage) getWorkerRef(id string) (worker.Worker, string, error) {
	workerID, refID, err := parseWorkerRef(id)
	if err != nil {
		return nil, "", err
	}
	w, err := s.wc.Get(workerID)
	if err != nil {
		return nil, "", err
	}
	return w, refID, nil
}

func (s *cacheResultStorage) load(id string) (solver.Result, error) {
	w, refID, err := s.getWorkerRef(id)
	if err != nil {
		return nil, err
	}
	if refID == "" {
		return worker.NewWorkerRefResult(nil, w), nil
	}
	ref, err := w.LoadRef(refID)
	if err != nil {
		return nil, err
	}
	return worker.NewWorkerRefResult(ref, w), nil
}

func (s *cacheResultStorage) LoadRemote(ctx context.Context, res solver.CacheResult) (*solver.Remote, error) {
	w, refID, err := s.getWorkerRef(res.ID)
	if err != nil {
		return nil, err
	}
	ref, err := w.LoadRef(refID)
	if err != nil {
		return nil, err
	}
	defer ref.Release(context.TODO())
	remote, err := w.GetRemote(ctx, ref, false)
	if err != nil {
		return nil, nil // ignore error. loadRemote is best effort
	}
	return remote, nil
}
func (s *cacheResultStorage) Exists(id string) bool {
	ref, err := s.load(id)
	if err != nil {
		return false
	}
	ref.Release(context.TODO())
	return true
}

func parseWorkerRef(id string) (string, string, error) {
	parts := strings.Split(id, "::")
	if len(parts) != 2 {
		return "", "", errors.Errorf("invalid workerref id: %s", id)
	}
	return parts[0], parts[1], nil
}

func workerRefConverter(ctx context.Context, res solver.Result) (*solver.Remote, error) {
	ref, ok := res.Sys().(*worker.WorkerRef)
	if !ok {
		return nil, errors.Errorf("invalid result: %T", res.Sys())
	}

	return ref.Worker.GetRemote(ctx, ref.ImmutableRef, true)
}
