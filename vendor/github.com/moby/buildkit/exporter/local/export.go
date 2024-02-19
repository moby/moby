package local

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/exporter/util/epoch"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/util/progress"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	fstypes "github.com/tonistiigi/fsutil/types"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

type Opt struct {
	SessionManager *session.Manager
}

type localExporter struct {
	opt Opt
	// session manager
}

func New(opt Opt) (exporter.Exporter, error) {
	le := &localExporter{opt: opt}
	return le, nil
}

func (e *localExporter) Resolve(ctx context.Context, id int, opt map[string]string) (exporter.ExporterInstance, error) {
	i := &localExporterInstance{
		id:            id,
		localExporter: e,
	}
	_, err := i.opts.Load(opt)
	if err != nil {
		return nil, err
	}

	return i, nil
}

type localExporterInstance struct {
	*localExporter
	id int

	opts CreateFSOpts
}

func (e *localExporterInstance) ID() int {
	return e.id
}

func (e *localExporterInstance) Name() string {
	return "exporting to client directory"
}

func (e *localExporter) Config() *exporter.Config {
	return exporter.NewConfig()
}

func (e *localExporterInstance) Export(ctx context.Context, inp *exporter.Source, _ exptypes.InlineCache, sessionID string) (map[string]string, exporter.DescriptorReference, error) {
	timeoutCtx, cancel := context.WithCancelCause(ctx)
	timeoutCtx, _ = context.WithTimeoutCause(timeoutCtx, 5*time.Second, errors.WithStack(context.DeadlineExceeded))
	defer cancel(errors.WithStack(context.Canceled))

	if e.opts.Epoch == nil {
		if tm, ok, err := epoch.ParseSource(inp); err != nil {
			return nil, nil, err
		} else if ok {
			e.opts.Epoch = tm
		}
	}

	caller, err := e.opt.SessionManager.Get(timeoutCtx, sessionID, false)
	if err != nil {
		return nil, nil, err
	}

	isMap := len(inp.Refs) > 0

	if _, ok := inp.Metadata[exptypes.ExporterPlatformsKey]; isMap && !ok {
		return nil, nil, errors.Errorf("unable to export multiple refs, missing platforms mapping")
	}
	p, err := exptypes.ParsePlatforms(inp.Metadata)
	if err != nil {
		return nil, nil, err
	}

	if !isMap && len(p.Platforms) > 1 {
		return nil, nil, errors.Errorf("unable to export multiple platforms without map")
	}

	now := time.Now().Truncate(time.Second)

	visitedPath := map[string]string{}
	var visitedMu sync.Mutex

	export := func(ctx context.Context, k string, ref cache.ImmutableRef, attestations []exporter.Attestation) func() error {
		return func() error {
			outputFS, cleanup, err := CreateFS(ctx, sessionID, k, ref, attestations, now, e.opts)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}

			if !e.opts.PlatformSplit {
				// check for duplicate paths
				err = outputFS.Walk(ctx, "", func(p string, entry os.DirEntry, err error) error {
					if entry.IsDir() {
						return nil
					}
					if err != nil && !errors.Is(err, os.ErrNotExist) {
						return err
					}
					visitedMu.Lock()
					defer visitedMu.Unlock()
					if vp, ok := visitedPath[p]; ok {
						return errors.Errorf("cannot overwrite %s from %s with %s when split option is disabled", p, vp, k)
					}
					visitedPath[p] = k
					return nil
				})
				if err != nil {
					return err
				}
			}

			lbl := "copying files"
			if isMap {
				lbl += " " + k
				if e.opts.PlatformSplit {
					st := fstypes.Stat{
						Mode: uint32(os.ModeDir | 0755),
						Path: strings.Replace(k, "/", "_", -1),
					}
					if e.opts.Epoch != nil {
						st.ModTime = e.opts.Epoch.UnixNano()
					}
					outputFS, err = fsutil.SubDirFS([]fsutil.Dir{{FS: outputFS, Stat: st}})
					if err != nil {
						return err
					}
				}
			}

			progress := NewProgressHandler(ctx, lbl)
			if err := filesync.CopyToCaller(ctx, outputFS, e.id, caller, progress); err != nil {
				return err
			}
			return nil
		}
	}

	eg, ctx := errgroup.WithContext(ctx)

	if len(p.Platforms) > 0 {
		for _, p := range p.Platforms {
			r, ok := inp.FindRef(p.ID)
			if !ok {
				return nil, nil, errors.Errorf("failed to find ref for ID %s", p.ID)
			}
			eg.Go(export(ctx, p.ID, r, inp.Attestations[p.ID]))
		}
	} else {
		eg.Go(export(ctx, "", inp.Ref, nil))
	}

	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}
	return nil, nil, nil
}

func NewProgressHandler(ctx context.Context, id string) func(int, bool) {
	limiter := rate.NewLimiter(rate.Every(100*time.Millisecond), 1)
	pw, _, _ := progress.NewFromContext(ctx)
	now := time.Now()
	st := progress.Status{
		Started: &now,
		Action:  "transferring",
	}
	pw.Write(id, st)
	return func(s int, last bool) {
		if last || limiter.Allow() {
			st.Current = s
			if last {
				now := time.Now()
				st.Completed = &now
			}
			pw.Write(id, st)
			if last {
				pw.Close()
			}
		}
	}
}
