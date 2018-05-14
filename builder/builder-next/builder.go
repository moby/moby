package buildkit

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/pkg/jsonmessage"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/control"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
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
	controller *control.Controller

	mu   sync.Mutex
	jobs map[string]func()
}

func New(opt Opt) (*Builder, error) {
	c, err := newController(opt)
	if err != nil {
		return nil, err
	}
	b := &Builder{
		controller: c,
		jobs:       map[string]func(){},
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

func (b *Builder) Build(ctx context.Context, opt backend.BuildConfig) (*builder.Result, error) {
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
		frontendAttrs["context"] = opt.Options.RemoteContext
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
		}, &statusProxy{ctx: ctx, ch: ch})
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

type statusProxy struct {
	ctx context.Context
	ch  chan *controlapi.StatusResponse
}

func (sp *statusProxy) SetHeader(_ grpcmetadata.MD) error {
	return nil
}

func (sp *statusProxy) SendHeader(_ grpcmetadata.MD) error {
	return nil
}

func (sp *statusProxy) SetTrailer(_ grpcmetadata.MD) {
}

func (sp *statusProxy) Send(resp *controlapi.StatusResponse) error {
	return sp.SendMsg(resp)
}

func (sp *statusProxy) Context() context.Context {
	return sp.ctx
}
func (sp *statusProxy) SendMsg(m interface{}) error {
	if sr, ok := m.(*controlapi.StatusResponse); ok {
		sp.ch <- sr
	}
	return nil
}
func (sp *statusProxy) RecvMsg(m interface{}) error {
	return io.EOF
}

type contentStoreNoLabels struct {
	content.Store
}

func (c *contentStoreNoLabels) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	return content.Info{}, nil
}
