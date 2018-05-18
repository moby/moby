package buildkit

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/pkg/jsonmessage"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/control"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/tracing"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	grpcmetadata "google.golang.org/grpc/metadata"
)

type Opt struct {
	SessionManager *session.Manager
	Root           string
	Dist           images.DistributionServices
}

type Builder struct {
	controller     *control.Controller
	reqBodyHandler *reqBodyHandler

	mu   sync.Mutex
	jobs map[string]func()
}

func New(opt Opt) (*Builder, error) {
	reqHandler := newReqBodyHandler(tracing.DefaultTransport)

	c, err := newController(reqHandler, opt)
	if err != nil {
		return nil, err
	}
	b := &Builder{
		controller:     c,
		reqBodyHandler: reqHandler,
		jobs:           map[string]func(){},
	}
	return b, nil
}

func (b *Builder) Cancel(ctx context.Context, id string) error {
	b.mu.Lock()
	if cancel, ok := b.jobs[id]; ok {
		cancel()
	}
	b.mu.Unlock()
	return nil
}

func (b *Builder) DiskUsage(ctx context.Context) ([]*types.BuildCache, error) {
	duResp, err := b.controller.DiskUsage(ctx, &controlapi.DiskUsageRequest{})
	if err != nil {
		return nil, err
	}

	var items []*types.BuildCache
	for _, r := range duResp.Record {
		items = append(items, &types.BuildCache{
			ID:      r.ID,
			Mutable: r.Mutable,
			InUse:   r.InUse,
			Size:    r.Size_,

			CreatedAt:   r.CreatedAt,
			LastUsedAt:  r.LastUsedAt,
			UsageCount:  int(r.UsageCount),
			Parent:      r.Parent,
			Description: r.Description,
		})
	}
	return items, nil
}

func (b *Builder) Prune(ctx context.Context) (int64, error) {
	ch := make(chan *controlapi.UsageRecord)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		defer close(ch)
		return b.controller.Prune(&controlapi.PruneRequest{}, &pruneProxy{
			streamProxy: streamProxy{ctx: ctx},
			ch:          ch,
		})
	})

	var size int64
	eg.Go(func() error {
		for r := range ch {
			size += r.Size_
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return 0, err
	}

	return size, nil
}

func (b *Builder) Build(ctx context.Context, opt backend.BuildConfig) (*builder.Result, error) {
	if buildID := opt.Options.BuildID; buildID != "" {
		b.mu.Lock()
		ctx, b.jobs[buildID] = context.WithCancel(ctx)
		b.mu.Unlock()
		defer func() {
			delete(b.jobs, buildID)
		}()
	}

	var out builder.Result

	id := identity.NewID()

	frontendAttrs := map[string]string{}

	if opt.Options.Target != "" {
		frontendAttrs["target"] = opt.Options.Target
	}

	if opt.Options.Dockerfile != "" && opt.Options.Dockerfile != "." {
		frontendAttrs["filename"] = opt.Options.Dockerfile
	}

	if opt.Options.RemoteContext != "" {
		if opt.Options.RemoteContext != "client-session" {
			frontendAttrs["context"] = opt.Options.RemoteContext
		}
	} else {
		url, cancel := b.reqBodyHandler.newRequest(opt.Source)
		defer cancel()
		frontendAttrs["context"] = url
	}

	var cacheFrom []string
	for _, v := range opt.Options.CacheFrom {
		cacheFrom = append(cacheFrom, v)
	}
	frontendAttrs["cache-from"] = strings.Join(cacheFrom, ",")

	for k, v := range opt.Options.BuildArgs {
		if v == nil {
			continue
		}
		frontendAttrs["build-arg:"+k] = *v
	}

	for k, v := range opt.Options.Labels {
		frontendAttrs["label:"+k] = v
	}

	if opt.Options.NoCache {
		frontendAttrs["no-cache"] = ""
	}

	req := &controlapi.SolveRequest{
		Ref:           id,
		Exporter:      "moby",
		Frontend:      "dockerfile.v0",
		FrontendAttrs: frontendAttrs,
		Session:       opt.Options.SessionID,
	}

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		resp, err := b.controller.Solve(ctx, req)
		if err != nil {
			return err
		}
		id, ok := resp.ExporterResponse["containerimage.digest"]
		if !ok {
			return errors.Errorf("missing image id")
		}
		out.ImageID = id
		return nil
	})

	ch := make(chan *controlapi.StatusResponse)

	eg.Go(func() error {
		defer close(ch)
		return b.controller.Status(&controlapi.StatusRequest{
			Ref: id,
		}, &statusProxy{streamProxy: streamProxy{ctx: ctx}, ch: ch})
	})

	eg.Go(func() error {
		for sr := range ch {
			dt, err := sr.Marshal()
			if err != nil {
				return err
			}

			auxJSONBytes, err := json.Marshal(dt)
			if err != nil {
				return err
			}
			auxJSON := new(json.RawMessage)
			*auxJSON = auxJSONBytes
			msgJSON, err := json.Marshal(&jsonmessage.JSONMessage{ID: "buildkit-trace", Aux: auxJSON})
			if err != nil {
				return err
			}
			msgJSON = append(msgJSON, []byte("\r\n")...)
			n, err := opt.ProgressWriter.Output.Write(msgJSON)
			if err != nil {
				return err
			}
			if n != len(msgJSON) {
				return io.ErrShortWrite
			}
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &out, nil
}

type streamProxy struct {
	ctx context.Context
}

func (sp *streamProxy) SetHeader(_ grpcmetadata.MD) error {
	return nil
}

func (sp *streamProxy) SendHeader(_ grpcmetadata.MD) error {
	return nil
}

func (sp *streamProxy) SetTrailer(_ grpcmetadata.MD) {
}

func (sp *streamProxy) Context() context.Context {
	return sp.ctx
}
func (sp *streamProxy) RecvMsg(m interface{}) error {
	return io.EOF
}

type statusProxy struct {
	streamProxy
	ch chan *controlapi.StatusResponse
}

func (sp *statusProxy) Send(resp *controlapi.StatusResponse) error {
	return sp.SendMsg(resp)
}
func (sp *statusProxy) SendMsg(m interface{}) error {
	if sr, ok := m.(*controlapi.StatusResponse); ok {
		sp.ch <- sr
	}
	return nil
}

type pruneProxy struct {
	streamProxy
	ch chan *controlapi.UsageRecord
}

func (sp *pruneProxy) Send(resp *controlapi.UsageRecord) error {
	return sp.SendMsg(resp)
}
func (sp *pruneProxy) SendMsg(m interface{}) error {
	if sr, ok := m.(*controlapi.UsageRecord); ok {
		sp.ch <- sr
	}
	return nil
}

type contentStoreNoLabels struct {
	content.Store
}

func (c *contentStoreNoLabels) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	return content.Info{}, nil
}
