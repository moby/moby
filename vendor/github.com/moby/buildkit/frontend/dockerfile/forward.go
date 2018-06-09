package dockerfile

import (
	"context"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/worker"
	"github.com/pkg/errors"
)

func llbBridgeToGatewayClient(ctx context.Context, llbBridge frontend.FrontendLLBBridge, opts map[string]string) (*bridgeClient, error) {
	return &bridgeClient{opts: opts, FrontendLLBBridge: llbBridge, sid: session.FromContext(ctx)}, nil
}

type bridgeClient struct {
	frontend.FrontendLLBBridge
	opts         map[string]string
	final        *ref
	sid          string
	exporterAttr map[string][]byte
	refs         []*ref
}

func (c *bridgeClient) Solve(ctx context.Context, req client.SolveRequest, exporterAttr map[string][]byte, final bool) (client.Reference, error) {
	r, exporterAttrRes, err := c.FrontendLLBBridge.Solve(ctx, frontend.SolveRequest{
		Definition:      req.Definition,
		Frontend:        req.Frontend,
		FrontendOpt:     req.FrontendOpt,
		ImportCacheRefs: req.ImportCacheRefs,
	})
	if err != nil {
		return nil, err
	}
	rr := &ref{r}
	c.refs = append(c.refs, rr)
	if final {
		c.final = rr
		if exporterAttr == nil {
			exporterAttr = make(map[string][]byte)
		}
		for k, v := range exporterAttrRes {
			exporterAttr[k] = v
		}
		c.exporterAttr = exporterAttr
	}
	return rr, nil
}
func (c *bridgeClient) Opts() map[string]string {
	return c.opts
}
func (c *bridgeClient) SessionID() string {
	return c.sid
}

type ref struct {
	solver.CachedResult
}

func (r *ref) ReadFile(ctx context.Context, req client.ReadRequest) ([]byte, error) {
	ref, err := r.getImmutableRef()
	if err != nil {
		return nil, err
	}
	newReq := cache.ReadRequest{
		Filename: req.Filename,
	}
	if r := req.Range; r != nil {
		newReq.Range = &cache.FileRange{
			Offset: r.Offset,
			Length: r.Length,
		}
	}
	return cache.ReadFile(ctx, ref, newReq)
}

func (r *ref) getImmutableRef() (cache.ImmutableRef, error) {
	ref, ok := r.CachedResult.Sys().(*worker.WorkerRef)
	if !ok {
		return nil, errors.Errorf("invalid ref: %T", r.CachedResult.Sys())
	}
	return ref.ImmutableRef, nil
}
