package local

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/exporter/util/epoch"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/staticfs"
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
		attrs:         opt,
		localExporter: e,
	}
	_, err := i.opts.Load(opt)
	if err != nil {
		return nil, err
	}

	return i, nil
}

func (e *localExporter) Config() *exporter.Config {
	return exporter.NewConfig()
}

type localExporterInstance struct {
	*localExporter
	id    int
	attrs map[string]string

	opts CreateFSOpts
}

func (e *localExporterInstance) ID() int {
	return e.id
}

func (e *localExporterInstance) Name() string {
	return "exporting to client directory"
}

func (e *localExporterInstance) Type() string {
	return client.ExporterLocal
}

func (e *localExporterInstance) Attrs() map[string]string {
	return e.attrs
}

func (e *localExporterInstance) Export(ctx context.Context, inp *exporter.Source, buildInfo exporter.ExportBuildInfo) (map[string]string, exporter.FinalizeFunc, exporter.DescriptorReference, error) {
	timeoutCtx, cancel := context.WithCancelCause(ctx)
	timeoutCtx, _ = context.WithTimeoutCause(timeoutCtx, 5*time.Second, errors.WithStack(context.DeadlineExceeded)) //nolint:govet
	defer func() { cancel(errors.WithStack(context.Canceled)) }()

	if e.opts.Epoch == nil {
		if tm, err := epoch.ParseSource(inp, nil); err != nil {
			return nil, nil, nil, err
		} else if tm != nil {
			e.opts.Epoch = &epoch.Epoch{Value: tm}
		}
	}

	caller, err := e.opt.SessionManager.Get(timeoutCtx, buildInfo.SessionID, false)
	if err != nil {
		return nil, nil, nil, err
	}

	isMap := len(inp.Refs) > 0

	if _, ok := inp.Metadata[exptypes.ExporterPlatformsKey]; isMap && !ok {
		return nil, nil, nil, errors.Errorf("unable to export multiple refs, missing platforms mapping")
	}
	platforms, err := exptypes.ParsePlatforms(inp.Metadata)
	if err != nil {
		return nil, nil, nil, err
	}

	if !isMap && len(platforms.Platforms) > 1 {
		return nil, nil, nil, errors.Errorf("unable to export multiple platforms without map")
	}

	now := time.Now().Truncate(time.Second)

	visitedPath := map[string]string{}
	var visitedMu sync.Mutex

	mode := client.LocalExporterModeCopy
	if e.attrs != nil {
		mode, err = client.ParseLocalExporterMode(e.attrs[keyMode])
		if err != nil {
			return nil, nil, nil, err
		}
	}

	platformDirStat := func(k string, opt CreateFSOpts) *fstypes.Stat {
		st := &fstypes.Stat{
			Mode: uint32(os.ModeDir | 0755),
			Path: strings.ReplaceAll(k, "/", "_"),
		}
		if opt.Epoch != nil && opt.Epoch.Value != nil {
			st.ModTime = opt.Epoch.Value.UnixNano()
		}
		return st
	}

	buildFS := func(ctx context.Context, k string, ref cache.ImmutableRef, attestations []exporter.Attestation, opt CreateFSOpts, wrapPlatformSplit bool) (fsutil.FS, func() error, string, error) {
		outputFS, cleanup, err := CreateFS(ctx, buildInfo.SessionID, k, ref, attestations, now, isMap, opt)
		if err != nil {
			return nil, nil, "", err
		}
		releaseOnError := true
		defer func() {
			if releaseOnError && cleanup != nil {
				_ = cleanup()
			}
		}()

		lbl := "copying files"
		if !e.opts.UsePlatformSplit(isMap) {
			// check for duplicate paths
			err = fsWalk(ctx, outputFS, "", func(p string, entry os.DirEntry, err error) error {
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
				return nil, nil, "", err
			}
		} else {
			lbl += " " + k
			if wrapPlatformSplit {
				outputFS, err = fsutil.SubDirFS([]fsutil.Dir{{FS: outputFS, Stat: platformDirStat(k, opt)}})
				if err != nil {
					return nil, nil, "", err
				}
			}
		}

		releaseOnError = false
		return outputFS, cleanup, lbl, nil
	}

	export := func(ctx context.Context, k string, ref cache.ImmutableRef, attestations []exporter.Attestation, opt CreateFSOpts) func() error {
		return func() error {
			outputFS, cleanup, lbl, err := buildFS(ctx, k, ref, attestations, opt, true)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}

			progress, closeProgress := NewProgressHandler(ctx, lbl)
			defer closeProgress()
			if err := filesync.CopyToCaller(ctx, outputFS, e.id, caller, progress); err != nil {
				return err
			}
			return nil
		}
	}

	eg, ctx := errgroup.WithContext(ctx)

	if mode == client.LocalExporterModeDelete {
		eg.Go(func() error {
			var outputFS fsutil.FS
			var platformDirs []fsutil.Dir
			var cleanups []func() error
			split := e.opts.UsePlatformSplit(isMap)
			defer func() {
				for i := len(cleanups) - 1; i >= 0; i-- {
					_ = cleanups[i]()
				}
			}()

			addFS := func(fs fsutil.FS) {
				if outputFS == nil {
					outputFS = fs
				} else {
					outputFS = staticfs.NewMergeFS(outputFS, fs)
				}
			}

			if len(platforms.Platforms) > 0 {
				for _, p := range platforms.Platforms {
					r, ok := inp.FindRef(p.ID)
					if !ok {
						return errors.Errorf("failed to find ref for ID %s", p.ID)
					}
					opt := e.opts
					if e.opts.Epoch == nil {
						tm, err := epoch.ParseSource(inp, &p)
						if err != nil {
							return err
						}
						opt.Epoch = &epoch.Epoch{Value: tm}
					}
					fs, cleanup, _, err := buildFS(ctx, p.ID, r, inp.Attestations[p.ID], opt, !split)
					if err != nil {
						return err
					}
					if cleanup != nil {
						cleanups = append(cleanups, cleanup)
					}
					if split {
						platformDirs = append(platformDirs, fsutil.Dir{FS: fs, Stat: platformDirStat(p.ID, opt)})
					} else {
						addFS(fs)
					}
				}
				if len(platformDirs) > 0 {
					fs, err := fsutil.SubDirFS(platformDirs)
					if err != nil {
						return err
					}
					addFS(fs)
				}
			} else {
				fs, cleanup, _, err := buildFS(ctx, "", inp.Ref, nil, e.opts, true)
				if err != nil {
					return err
				}
				if cleanup != nil {
					cleanups = append(cleanups, cleanup)
				}
				addFS(fs)
			}

			progress, closeProgress := NewProgressHandler(ctx, "copying files")
			defer closeProgress()
			return filesync.CopyToCaller(ctx, outputFS, e.id, caller, progress, filesync.WithExporterMultiPlatformTransfer())
		})
	} else if len(platforms.Platforms) > 0 {
		for _, p := range platforms.Platforms {
			r, ok := inp.FindRef(p.ID)
			if !ok {
				return nil, nil, nil, errors.Errorf("failed to find ref for ID %s", p.ID)
			}
			opt := e.opts
			if e.opts.Epoch == nil {
				tm, err := epoch.ParseSource(inp, &p)
				if err != nil {
					return nil, nil, nil, err
				}
				opt.Epoch = &epoch.Epoch{Value: tm}
			}
			eg.Go(export(ctx, p.ID, r, inp.Attestations[p.ID], opt))
		}
	} else {
		eg.Go(export(ctx, "", inp.Ref, nil, e.opts))
	}

	if err := eg.Wait(); err != nil {
		return nil, nil, nil, err
	}
	return nil, nil, nil, nil
}

// NewProgressHandler returns a callback for reporting transfer progress and a
// cleanup function that ensures the underlying progress writer is closed.
// The cleanup function must be called by the caller (typically via defer), so
// the writer is released even when the callback is never invoked with
// last=true (for example when an early error short-circuits the transfer).
func NewProgressHandler(ctx context.Context, id string) (func(int, bool), func()) {
	limiter := rate.NewLimiter(rate.Every(100*time.Millisecond), 1)
	pw, _, _ := progress.NewFromContext(ctx)
	now := time.Now()
	st := progress.Status{
		Started: &now,
		Action:  "transferring",
	}
	pw.Write(id, st)
	cb := func(s int, last bool) {
		if last || limiter.Allow() {
			st.Current = s
			if last {
				now := time.Now()
				st.Completed = &now
			}
			pw.Write(id, st)
		}
	}
	return cb, func() { pw.Close() }
}
