package client

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/session/grpchijack"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/entitlements"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type SolveOpt struct {
	Exporter            string
	ExporterAttrs       map[string]string
	ExporterOutput      io.WriteCloser // for ExporterOCI and ExporterDocker
	ExporterOutputDir   string         // for ExporterLocal
	LocalDirs           map[string]string
	SharedKey           string
	Frontend            string
	FrontendAttrs       map[string]string
	ExportCache         string
	ExportCacheAttrs    map[string]string
	ImportCache         []string
	Session             []session.Attachable
	AllowedEntitlements []entitlements.Entitlement
}

// Solve calls Solve on the controller.
// def must be nil if (and only if) opt.Frontend is set.
func (c *Client) Solve(ctx context.Context, def *llb.Definition, opt SolveOpt, statusChan chan *SolveStatus) (*SolveResponse, error) {
	defer func() {
		if statusChan != nil {
			close(statusChan)
		}
	}()

	if opt.Frontend == "" && def == nil {
		return nil, errors.New("invalid empty definition")
	}
	if opt.Frontend != "" && def != nil {
		return nil, errors.Errorf("invalid definition for frontend %s", opt.Frontend)
	}

	return c.solve(ctx, def, nil, opt, statusChan)
}

type runGatewayCB func(ref string, s *session.Session) error

func (c *Client) solve(ctx context.Context, def *llb.Definition, runGateway runGatewayCB, opt SolveOpt, statusChan chan *SolveStatus) (*SolveResponse, error) {
	if def != nil && runGateway != nil {
		return nil, errors.New("invalid with def and cb")
	}

	syncedDirs, err := prepareSyncedDirs(def, opt.LocalDirs)
	if err != nil {
		return nil, err
	}

	ref := identity.NewID()
	eg, ctx := errgroup.WithContext(ctx)

	statusContext, cancelStatus := context.WithCancel(context.Background())
	defer cancelStatus()

	if span := opentracing.SpanFromContext(ctx); span != nil {
		statusContext = opentracing.ContextWithSpan(statusContext, span)
	}

	s, err := session.NewSession(statusContext, defaultSessionName(), opt.SharedKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create session")
	}

	if len(syncedDirs) > 0 {
		s.Allow(filesync.NewFSSyncProvider(syncedDirs))
	}

	for _, a := range opt.Session {
		s.Allow(a)
	}

	switch opt.Exporter {
	case ExporterLocal:
		if opt.ExporterOutput != nil {
			return nil, errors.New("output file writer is not supported by local exporter")
		}
		if opt.ExporterOutputDir == "" {
			return nil, errors.New("output directory is required for local exporter")
		}
		s.Allow(filesync.NewFSSyncTargetDir(opt.ExporterOutputDir))
	case ExporterOCI, ExporterDocker:
		if opt.ExporterOutputDir != "" {
			return nil, errors.Errorf("output directory %s is not supported by %s exporter", opt.ExporterOutputDir, opt.Exporter)
		}
		if opt.ExporterOutput == nil {
			return nil, errors.Errorf("output file writer is required for %s exporter", opt.Exporter)
		}
		s.Allow(filesync.NewFSSyncTarget(opt.ExporterOutput))
	default:
		if opt.ExporterOutput != nil {
			return nil, errors.Errorf("output file writer is not supported by %s exporter", opt.Exporter)
		}
		if opt.ExporterOutputDir != "" {
			return nil, errors.Errorf("output directory %s is not supported by %s exporter", opt.ExporterOutputDir, opt.Exporter)
		}
	}

	eg.Go(func() error {
		return s.Run(statusContext, grpchijack.Dialer(c.controlClient()))
	})

	solveCtx, cancelSolve := context.WithCancel(ctx)
	var res *SolveResponse
	eg.Go(func() error {
		ctx := solveCtx
		defer cancelSolve()

		defer func() { // make sure the Status ends cleanly on build errors
			go func() {
				<-time.After(3 * time.Second)
				cancelStatus()
			}()
			logrus.Debugf("stopping session")
			s.Close()
		}()
		var pbd *pb.Definition
		if def != nil {
			pbd = def.ToPB()
		}
		resp, err := c.controlClient().Solve(ctx, &controlapi.SolveRequest{
			Ref:           ref,
			Definition:    pbd,
			Exporter:      opt.Exporter,
			ExporterAttrs: opt.ExporterAttrs,
			Session:       s.ID(),
			Frontend:      opt.Frontend,
			FrontendAttrs: opt.FrontendAttrs,
			Cache: controlapi.CacheOptions{
				ExportRef:   opt.ExportCache,
				ImportRefs:  opt.ImportCache,
				ExportAttrs: opt.ExportCacheAttrs,
			},
			Entitlements: opt.AllowedEntitlements,
		})
		if err != nil {
			return errors.Wrap(err, "failed to solve")
		}
		res = &SolveResponse{
			ExporterResponse: resp.ExporterResponse,
		}
		return nil
	})

	if runGateway != nil {
		eg.Go(func() error {
			err := runGateway(ref, s)
			if err == nil {
				return nil
			}

			// If the callback failed then the main
			// `Solve` (called above) should error as
			// well. However as a fallback we wait up to
			// 5s for that to happen before failing this
			// goroutine.
			select {
			case <-solveCtx.Done():
			case <-time.After(5 * time.Second):
				cancelSolve()
			}

			return err
		})
	}

	eg.Go(func() error {
		stream, err := c.controlClient().Status(statusContext, &controlapi.StatusRequest{
			Ref: ref,
		})
		if err != nil {
			return errors.Wrap(err, "failed to get status")
		}
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return errors.Wrap(err, "failed to receive status")
			}
			s := SolveStatus{}
			for _, v := range resp.Vertexes {
				s.Vertexes = append(s.Vertexes, &Vertex{
					Digest:    v.Digest,
					Inputs:    v.Inputs,
					Name:      v.Name,
					Started:   v.Started,
					Completed: v.Completed,
					Error:     v.Error,
					Cached:    v.Cached,
				})
			}
			for _, v := range resp.Statuses {
				s.Statuses = append(s.Statuses, &VertexStatus{
					ID:        v.ID,
					Vertex:    v.Vertex,
					Name:      v.Name,
					Total:     v.Total,
					Current:   v.Current,
					Timestamp: v.Timestamp,
					Started:   v.Started,
					Completed: v.Completed,
				})
			}
			for _, v := range resp.Logs {
				s.Logs = append(s.Logs, &VertexLog{
					Vertex:    v.Vertex,
					Stream:    int(v.Stream),
					Data:      v.Msg,
					Timestamp: v.Timestamp,
				})
			}
			if statusChan != nil {
				statusChan <- &s
			}
		}
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return res, nil
}

func prepareSyncedDirs(def *llb.Definition, localDirs map[string]string) ([]filesync.SyncedDir, error) {
	for _, d := range localDirs {
		fi, err := os.Stat(d)
		if err != nil {
			return nil, errors.Wrapf(err, "could not find %s", d)
		}
		if !fi.IsDir() {
			return nil, errors.Errorf("%s not a directory", d)
		}
	}
	dirs := make([]filesync.SyncedDir, 0, len(localDirs))
	if def == nil {
		for name, d := range localDirs {
			dirs = append(dirs, filesync.SyncedDir{Name: name, Dir: d})
		}
	} else {
		for _, dt := range def.Def {
			var op pb.Op
			if err := (&op).Unmarshal(dt); err != nil {
				return nil, errors.Wrap(err, "failed to parse llb proto op")
			}
			if src := op.GetSource(); src != nil {
				if strings.HasPrefix(src.Identifier, "local://") { // TODO: just make a type property
					name := strings.TrimPrefix(src.Identifier, "local://")
					d, ok := localDirs[name]
					if !ok {
						return nil, errors.Errorf("local directory %s not enabled", name)
					}
					dirs = append(dirs, filesync.SyncedDir{Name: name, Dir: d}) // TODO: excludes
				}
			}
		}
	}
	return dirs, nil
}

func defaultSessionName() string {
	wd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return filepath.Base(wd)
}
