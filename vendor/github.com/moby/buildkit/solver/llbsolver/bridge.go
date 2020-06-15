package llbsolver

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/platforms"
	"github.com/mitchellh/hashstructure"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/frontend"
	gw "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/errdefs"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/moby/buildkit/util/tracing"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type llbBridge struct {
	builder                   solver.Builder
	frontends                 map[string]frontend.Frontend
	resolveWorker             func() (worker.Worker, error)
	eachWorker                func(func(worker.Worker) error) error
	resolveCacheImporterFuncs map[string]remotecache.ResolveCacheImporterFunc
	cms                       map[string]solver.CacheManager
	cmsMu                     sync.Mutex
	sm                        *session.Manager
}

func (b *llbBridge) loadResult(ctx context.Context, def *pb.Definition, cacheImports []gw.CacheOptionsEntry) (solver.CachedResult, error) {
	w, err := b.resolveWorker()
	if err != nil {
		return nil, err
	}
	ent, err := loadEntitlements(b.builder)
	if err != nil {
		return nil, err
	}
	var cms []solver.CacheManager
	for _, im := range cacheImports {
		cmID, err := cmKey(im)
		if err != nil {
			return nil, err
		}
		b.cmsMu.Lock()
		var cm solver.CacheManager
		if prevCm, ok := b.cms[cmID]; !ok {
			func(cmID string, im gw.CacheOptionsEntry) {
				cm = newLazyCacheManager(cmID, func() (solver.CacheManager, error) {
					var cmNew solver.CacheManager
					if err := inVertexContext(b.builder.Context(context.TODO()), "importing cache manifest from "+cmID, "", func(ctx context.Context) error {
						resolveCI, ok := b.resolveCacheImporterFuncs[im.Type]
						if !ok {
							return errors.Errorf("unknown cache importer: %s", im.Type)
						}
						ci, desc, err := resolveCI(ctx, im.Attrs)
						if err != nil {
							return err
						}
						cmNew, err = ci.Resolve(ctx, desc, cmID, w)
						return err
					}); err != nil {
						logrus.Debugf("error while importing cache manifest from cmId=%s: %v", cmID, err)
						return nil, err
					}
					return cmNew, nil
				})
			}(cmID, im)
			b.cms[cmID] = cm
		} else {
			cm = prevCm
		}
		cms = append(cms, cm)
		b.cmsMu.Unlock()
	}
	dpc := &detectPrunedCacheID{}

	edge, err := Load(def, dpc.Load, ValidateEntitlements(ent), WithCacheSources(cms), NormalizeRuntimePlatforms(), WithValidateCaps())
	if err != nil {
		return nil, errors.Wrap(err, "failed to load LLB")
	}

	if len(dpc.ids) > 0 {
		ids := make([]string, 0, len(dpc.ids))
		for id := range dpc.ids {
			ids = append(ids, id)
		}
		if err := b.eachWorker(func(w worker.Worker) error {
			return w.PruneCacheMounts(ctx, ids)
		}); err != nil {
			return nil, err
		}
	}

	res, err := b.builder.Build(ctx, edge)
	if err != nil {
		return nil, err
	}
	wr, ok := res.Sys().(*worker.WorkerRef)
	if !ok {
		return nil, errors.Errorf("invalid reference for exporting: %T", res.Sys())
	}
	if wr.ImmutableRef != nil {
		if err := wr.ImmutableRef.Finalize(ctx, false); err != nil {
			return nil, err
		}
	}
	return res, err
}

func (b *llbBridge) Solve(ctx context.Context, req frontend.SolveRequest) (res *frontend.Result, err error) {
	if req.Definition != nil && req.Definition.Def != nil && req.Frontend != "" {
		return nil, errors.New("cannot solve with both Definition and Frontend specified")
	}

	if req.Definition != nil && req.Definition.Def != nil {
		res = &frontend.Result{Ref: newResultProxy(b, req)}
	} else if req.Frontend != "" {
		f, ok := b.frontends[req.Frontend]
		if !ok {
			return nil, errors.Errorf("invalid frontend: %s", req.Frontend)
		}
		res, err = f.Solve(ctx, b, req.FrontendOpt, req.FrontendInputs)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to solve with frontend %s", req.Frontend)
		}
	} else {
		return &frontend.Result{}, nil
	}

	return
}

type resultProxy struct {
	cb       func(context.Context) (solver.CachedResult, error)
	def      *pb.Definition
	g        flightcontrol.Group
	mu       sync.Mutex
	released bool
	v        solver.CachedResult
	err      error
}

func newResultProxy(b *llbBridge, req frontend.SolveRequest) *resultProxy {
	return &resultProxy{
		def: req.Definition,
		cb: func(ctx context.Context) (solver.CachedResult, error) {
			return b.loadResult(ctx, req.Definition, req.CacheImports)
		},
	}
}

func (rp *resultProxy) Definition() *pb.Definition {
	return rp.def
}

func (rp *resultProxy) Release(ctx context.Context) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if rp.v != nil {
		if rp.released {
			logrus.Warnf("release of already released result")
		}
		if err := rp.v.Release(ctx); err != nil {
			return err
		}
	}
	rp.released = true
	return nil
}

func (rp *resultProxy) wrapError(err error) error {
	if err == nil {
		return nil
	}
	var ve *errdefs.VertexError
	if errors.As(err, &ve) {
		if rp.def.Source != nil {
			locs, ok := rp.def.Source.Locations[string(ve.Digest)]
			if ok {
				for _, loc := range locs.Locations {
					err = errdefs.WithSource(err, errdefs.Source{
						Info:   rp.def.Source.Infos[loc.SourceIndex],
						Ranges: loc.Ranges,
					})
				}
			}
		}
	}
	return err
}

func (rp *resultProxy) Result(ctx context.Context) (res solver.CachedResult, err error) {
	defer func() {
		err = rp.wrapError(err)
	}()
	r, err := rp.g.Do(ctx, "result", func(ctx context.Context) (interface{}, error) {
		rp.mu.Lock()
		if rp.released {
			rp.mu.Unlock()
			return nil, errors.Errorf("accessing released result")
		}
		if rp.v != nil || rp.err != nil {
			rp.mu.Unlock()
			return rp.v, rp.err
		}
		rp.mu.Unlock()
		v, err := rp.cb(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				if strings.Contains(err.Error(), context.Canceled.Error()) {
					return v, err
				}
			default:
			}
		}
		rp.mu.Lock()
		if rp.released {
			if v != nil {
				v.Release(context.TODO())
			}
			rp.mu.Unlock()
			return nil, errors.Errorf("evaluating released result")
		}
		rp.v = v
		rp.err = err
		rp.mu.Unlock()
		return v, err
	})
	if r != nil {
		return r.(solver.CachedResult), nil
	}
	return nil, err
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

func (s *llbBridge) ResolveImageConfig(ctx context.Context, ref string, opt llb.ResolveImageConfigOpt) (dgst digest.Digest, config []byte, err error) {
	w, err := s.resolveWorker()
	if err != nil {
		return "", nil, err
	}
	if opt.LogName == "" {
		opt.LogName = fmt.Sprintf("resolve image config for %s", ref)
	}
	id := ref // make a deterministic ID for avoiding duplicates
	if platform := opt.Platform; platform == nil {
		id += platforms.Format(platforms.DefaultSpec())
	} else {
		id += platforms.Format(*platform)
	}
	err = inVertexContext(s.builder.Context(ctx), opt.LogName, id, func(ctx context.Context) error {
		dgst, config, err = w.ResolveImageConfig(ctx, ref, opt, s.sm)
		return err
	})
	return dgst, config, err
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
	lcm.wait()
	if lcm.main == nil {
		return nil, nil
	}
	return lcm.main.Query(inp, inputIndex, dgst, outputIndex)
}
func (lcm *lazyCacheManager) Records(ck *solver.CacheKey) ([]*solver.CacheRecord, error) {
	lcm.wait()
	if lcm.main == nil {
		return nil, nil
	}
	return lcm.main.Records(ck)
}
func (lcm *lazyCacheManager) Load(ctx context.Context, rec *solver.CacheRecord) (solver.Result, error) {
	if err := lcm.wait(); err != nil {
		return nil, err
	}
	return lcm.main.Load(ctx, rec)
}
func (lcm *lazyCacheManager) Save(key *solver.CacheKey, s solver.Result, createdAt time.Time) (*solver.ExportableCacheKey, error) {
	if err := lcm.wait(); err != nil {
		return nil, err
	}
	return lcm.main.Save(key, s, createdAt)
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

func cmKey(im gw.CacheOptionsEntry) (string, error) {
	if im.Type == "registry" && im.Attrs["ref"] != "" {
		return im.Attrs["ref"], nil
	}
	i, err := hashstructure.Hash(im, nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", im.Type, i), nil
}
