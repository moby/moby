package llbsolver

import (
	"context"
	"io"
	"strings"
	"sync"

	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/tracing"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type llbBridge struct {
	builder       solver.Builder
	frontends     map[string]frontend.Frontend
	resolveWorker func() (worker.Worker, error)
	ci            *remotecache.CacheImporter
	cms           map[string]solver.CacheManager
	cmsMu         sync.Mutex
}

func (b *llbBridge) Solve(ctx context.Context, req frontend.SolveRequest) (res solver.CachedResult, exp map[string][]byte, err error) {
	var cms []solver.CacheManager
	for _, ref := range req.ImportCacheRefs {
		b.cmsMu.Lock()
		var cm solver.CacheManager
		if prevCm, ok := b.cms[ref]; !ok {
			r, err := reference.ParseNormalizedNamed(ref)
			if err != nil {
				return nil, nil, err
			}
			ref = reference.TagNameOnly(r).String()
			func(ref string) {
				cm = newLazyCacheManager(ref, func() (solver.CacheManager, error) {
					var cmNew solver.CacheManager
					if err := b.builder.Call(ctx, "importing cache manifest from "+ref, func(ctx context.Context) error {
						cmNew, err = b.ci.Resolve(ctx, ref)
						return err
					}); err != nil {
						return nil, err
					}
					return cmNew, nil
				})
			}(ref)
			b.cms[ref] = cm
		} else {
			cm = prevCm
		}
		cms = append(cms, cm)
		b.cmsMu.Unlock()
	}

	if req.Definition != nil && req.Definition.Def != nil {
		edge, err := Load(req.Definition, WithCacheSources(cms))
		if err != nil {
			return nil, nil, err
		}
		res, err = b.builder.Build(ctx, edge)
		if err != nil {
			return nil, nil, err
		}
	}
	if req.Frontend != "" {
		f, ok := b.frontends[req.Frontend]
		if !ok {
			return nil, nil, errors.Errorf("invalid frontend: %s", req.Frontend)
		}
		res, exp, err = f.Solve(ctx, b, req.FrontendOpt)
		if err != nil {
			return nil, nil, err
		}
	} else {
		if req.Definition == nil || req.Definition.Def == nil {
			return nil, nil, nil
		}
	}

	if res != nil {
		wr, ok := res.Sys().(*worker.WorkerRef)
		if !ok {
			return nil, nil, errors.Errorf("invalid reference for exporting: %T", res.Sys())
		}
		if wr.ImmutableRef != nil {
			if err := wr.ImmutableRef.Finalize(ctx); err != nil {
				return nil, nil, err
			}
		}
	}
	return
}

func (s *llbBridge) Exec(ctx context.Context, meta executor.Meta, root cache.ImmutableRef, stdin io.ReadCloser, stdout, stderr io.WriteCloser) (err error) {
	w, err := s.resolveWorker()
	if err != nil {
		return err
	}
	span, ctx := tracing.StartSpan(ctx, strings.Join(meta.Args, " "))
	err = w.Exec(ctx, meta, root, stdin, stdout, stderr)
	tracing.FinishWithError(span, err)
	return err
}

func (s *llbBridge) ResolveImageConfig(ctx context.Context, ref string) (digest.Digest, []byte, error) {
	w, err := s.resolveWorker()
	if err != nil {
		return "", nil, err
	}
	return w.ResolveImageConfig(ctx, ref)
}

type lazyCacheManager struct {
	id   string
	main solver.CacheManager

	waitCh chan struct{}
	err    error
}

func (lcm *lazyCacheManager) ID() string {
	return lcm.id
}
func (lcm *lazyCacheManager) Query(inp []solver.CacheKeyWithSelector, inputIndex solver.Index, dgst digest.Digest, outputIndex solver.Index) ([]*solver.CacheKey, error) {
	if err := lcm.wait(); err != nil {
		return nil, err
	}
	return lcm.main.Query(inp, inputIndex, dgst, outputIndex)
}
func (lcm *lazyCacheManager) Records(ck *solver.CacheKey) ([]*solver.CacheRecord, error) {
	if err := lcm.wait(); err != nil {
		return nil, err
	}
	return lcm.main.Records(ck)
}
func (lcm *lazyCacheManager) Load(ctx context.Context, rec *solver.CacheRecord) (solver.Result, error) {
	if err := lcm.wait(); err != nil {
		return nil, err
	}
	return lcm.main.Load(ctx, rec)
}
func (lcm *lazyCacheManager) Save(key *solver.CacheKey, s solver.Result) (*solver.ExportableCacheKey, error) {
	if err := lcm.wait(); err != nil {
		return nil, err
	}
	return lcm.main.Save(key, s)
}

func (lcm *lazyCacheManager) wait() error {
	<-lcm.waitCh
	return lcm.err
}

func newLazyCacheManager(id string, fn func() (solver.CacheManager, error)) solver.CacheManager {
	lcm := &lazyCacheManager{id: id, waitCh: make(chan struct{})}
	go func() {
		defer close(lcm.waitCh)
		cm, err := fn()
		if err != nil {
			lcm.err = err
			return
		}
		lcm.main = cm
	}()
	return lcm
}
