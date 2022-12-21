package local

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/exporter/local"
	"github.com/moby/buildkit/exporter/util/epoch"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/util/progress"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	fstypes "github.com/tonistiigi/fsutil/types"
)

const (
	attestationPrefixKey = "attestation-prefix"

	// preferNondistLayersKey is an exporter option which can be used to mark a layer as non-distributable if the layer reference was
	// already found to use a non-distributable media type.
	// When this option is not set, the exporter will change the media type of the layer to a distributable one.
	preferNondistLayersKey = "prefer-nondist-layers"
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

func (e *localExporter) Resolve(ctx context.Context, opt map[string]string) (exporter.ExporterInstance, error) {
	li := &localExporterInstance{localExporter: e}

	tm, opt, err := epoch.ParseExporterAttrs(opt)
	if err != nil {
		return nil, err
	}
	li.opts.Epoch = tm

	for k, v := range opt {
		switch k {
		case preferNondistLayersKey:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value for %s: %s", preferNondistLayersKey, v)
			}
			li.preferNonDist = b
		case attestationPrefixKey:
			li.opts.AttestationPrefix = v
		}
	}

	return li, nil
}

type localExporterInstance struct {
	*localExporter
	opts          local.CreateFSOpts
	preferNonDist bool
}

func (e *localExporterInstance) Name() string {
	return "exporting to client tarball"
}

func (e *localExporterInstance) Config() *exporter.Config {
	return exporter.NewConfig()
}

func (e *localExporterInstance) Export(ctx context.Context, inp *exporter.Source, sessionID string) (map[string]string, exporter.DescriptorReference, error) {
	var defers []func() error

	defer func() {
		for i := len(defers) - 1; i >= 0; i-- {
			defers[i]()
		}
	}()

	if e.opts.Epoch == nil {
		if tm, ok, err := epoch.ParseSource(inp); err != nil {
			return nil, nil, err
		} else if ok {
			e.opts.Epoch = tm
		}
	}

	now := time.Now().Truncate(time.Second)

	getDir := func(ctx context.Context, k string, ref cache.ImmutableRef, attestations []exporter.Attestation) (*fsutil.Dir, error) {
		outputFS, cleanup, err := local.CreateFS(ctx, sessionID, k, ref, attestations, now, e.opts)
		if err != nil {
			return nil, err
		}
		if cleanup != nil {
			defers = append(defers, cleanup)
		}

		st := fstypes.Stat{
			Mode: uint32(os.ModeDir | 0755),
			Path: strings.Replace(k, "/", "_", -1),
		}
		if e.opts.Epoch != nil {
			st.ModTime = e.opts.Epoch.UnixNano()
		}

		return &fsutil.Dir{
			FS:   outputFS,
			Stat: st,
		}, nil
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

	var fs fsutil.FS

	if len(p.Platforms) > 0 {
		dirs := make([]fsutil.Dir, 0, len(p.Platforms))
		for _, p := range p.Platforms {
			r, ok := inp.FindRef(p.ID)
			if !ok {
				return nil, nil, errors.Errorf("failed to find ref for ID %s", p.ID)
			}
			d, err := getDir(ctx, p.ID, r, inp.Attestations[p.ID])
			if err != nil {
				return nil, nil, err
			}
			dirs = append(dirs, *d)
		}
		if isMap {
			var err error
			fs, err = fsutil.SubDirFS(dirs)
			if err != nil {
				return nil, nil, err
			}
		} else {
			fs = dirs[0].FS
		}
	} else {
		d, err := getDir(ctx, "", inp.Ref, nil)
		if err != nil {
			return nil, nil, err
		}
		fs = d.FS
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	caller, err := e.opt.SessionManager.Get(timeoutCtx, sessionID, false)
	if err != nil {
		return nil, nil, err
	}

	w, err := filesync.CopyFileWriter(ctx, nil, caller)
	if err != nil {
		return nil, nil, err
	}
	report := progress.OneOff(ctx, "sending tarball")
	if err := fsutil.WriteTar(ctx, fs, w); err != nil {
		w.Close()
		return nil, nil, report(err)
	}
	return nil, nil, report(w.Close())
}
