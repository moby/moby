package forwarder

import (
	"context"
	"sync"

	cacheutil "github.com/moby/buildkit/cache/util"
	clienttypes "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/frontend/gateway"
	"github.com/moby/buildkit/frontend/gateway/client"
	gwpb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	opspb "github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/apicaps"
	"github.com/moby/buildkit/worker"
	"github.com/pkg/errors"
	fstypes "github.com/tonistiigi/fsutil/types"
)

func llbBridgeToGatewayClient(ctx context.Context, llbBridge frontend.FrontendLLBBridge, opts map[string]string, inputs map[string]*opspb.Definition, workerInfos []clienttypes.WorkerInfo, sid string, sm *session.Manager) (*bridgeClient, error) {
	return &bridgeClient{
		opts:              opts,
		inputs:            inputs,
		FrontendLLBBridge: llbBridge,
		sid:               sid,
		sm:                sm,
		workerInfos:       workerInfos,
		final:             map[*ref]struct{}{},
	}, nil
}

type bridgeClient struct {
	frontend.FrontendLLBBridge
	mu          sync.Mutex
	opts        map[string]string
	inputs      map[string]*opspb.Definition
	final       map[*ref]struct{}
	sid         string
	sm          *session.Manager
	refs        []*ref
	workerInfos []clienttypes.WorkerInfo
}

func (c *bridgeClient) Solve(ctx context.Context, req client.SolveRequest) (*client.Result, error) {
	res, err := c.FrontendLLBBridge.Solve(ctx, frontend.SolveRequest{
		Definition:     req.Definition,
		Frontend:       req.Frontend,
		FrontendOpt:    req.FrontendOpt,
		FrontendInputs: req.FrontendInputs,
		CacheImports:   req.CacheImports,
	}, c.sid)
	if err != nil {
		return nil, err
	}

	cRes := &client.Result{}
	c.mu.Lock()
	for k, r := range res.Refs {
		rr, err := newRef(r, session.NewGroup(c.sid))
		if err != nil {
			return nil, err
		}
		c.refs = append(c.refs, rr)
		cRes.AddRef(k, rr)
	}
	if r := res.Ref; r != nil {
		rr, err := newRef(r, session.NewGroup(c.sid))
		if err != nil {
			return nil, err
		}
		c.refs = append(c.refs, rr)
		cRes.SetRef(rr)
	}
	c.mu.Unlock()
	cRes.Metadata = res.Metadata

	return cRes, nil
}
func (c *bridgeClient) BuildOpts() client.BuildOpts {
	workers := make([]client.WorkerInfo, 0, len(c.workerInfos))
	for _, w := range c.workerInfos {
		workers = append(workers, client.WorkerInfo{
			ID:        w.ID,
			Labels:    w.Labels,
			Platforms: w.Platforms,
		})
	}

	return client.BuildOpts{
		Opts:      c.opts,
		SessionID: c.sid,
		Workers:   workers,
		Product:   apicaps.ExportedProduct,
		Caps:      gwpb.Caps.CapSet(gwpb.Caps.All()),
		LLBCaps:   opspb.Caps.CapSet(opspb.Caps.All()),
	}
}

func (c *bridgeClient) Inputs(ctx context.Context) (map[string]llb.State, error) {
	inputs := make(map[string]llb.State)
	for key, def := range c.inputs {
		defop, err := llb.NewDefinitionOp(def)
		if err != nil {
			return nil, err
		}
		inputs[key] = llb.NewState(defop)
	}
	return inputs, nil
}

func (c *bridgeClient) toFrontendResult(r *client.Result) (*frontend.Result, error) {
	if r == nil {
		return nil, nil
	}

	res := &frontend.Result{}

	if r.Refs != nil {
		res.Refs = make(map[string]solver.ResultProxy, len(r.Refs))
		for k, r := range r.Refs {
			rr, ok := r.(*ref)
			if !ok {
				return nil, errors.Errorf("invalid reference type for forward %T", r)
			}
			c.final[rr] = struct{}{}
			res.Refs[k] = rr.ResultProxy
		}
	}
	if r := r.Ref; r != nil {
		rr, ok := r.(*ref)
		if !ok {
			return nil, errors.Errorf("invalid reference type for forward %T", r)
		}
		c.final[rr] = struct{}{}
		res.Ref = rr.ResultProxy
	}
	res.Metadata = r.Metadata

	return res, nil
}

func (c *bridgeClient) discard(err error) {
	for _, r := range c.refs {
		if r != nil {
			if _, ok := c.final[r]; !ok || err != nil {
				r.Release(context.TODO())
			}
		}
	}
}

func (c *bridgeClient) NewContainer(ctx context.Context, req client.NewContainerRequest) (client.Container, error) {
	ctrReq := gateway.NewContainerRequest{
		ContainerID: identity.NewID(),
		NetMode:     req.NetMode,
	}

	for _, m := range req.Mounts {
		var refProxy solver.ResultProxy
		if m.Ref != nil {
			var ok bool
			refProxy, ok = m.Ref.(*ref)
			if !ok {
				return nil, errors.Errorf("unexpected Ref type: %T", m.Ref)
			}
		}
		ctrReq.Mounts = append(ctrReq.Mounts, gateway.Mount{
			Dest:      m.Dest,
			Selector:  m.Selector,
			Readonly:  m.Readonly,
			MountType: m.MountType,
			RefProxy:  refProxy,
			CacheOpt:  m.CacheOpt,
			SecretOpt: m.SecretOpt,
			SSHOpt:    m.SSHOpt,
		})
	}

	group := session.NewGroup(c.sid)
	ctr, err := gateway.NewContainer(ctx, c, c.sm, group, ctrReq)
	if err != nil {
		return nil, err
	}
	return ctr, nil
}

type ref struct {
	solver.ResultProxy
	session session.Group
}

func newRef(r solver.ResultProxy, s session.Group) (*ref, error) {
	return &ref{ResultProxy: r, session: s}, nil
}

func (r *ref) ToState() (st llb.State, err error) {
	defop, err := llb.NewDefinitionOp(r.Definition())
	if err != nil {
		return st, err
	}
	return llb.NewState(defop), nil
}

func (r *ref) ReadFile(ctx context.Context, req client.ReadRequest) ([]byte, error) {
	m, err := r.getMountable(ctx)
	if err != nil {
		return nil, err
	}
	newReq := cacheutil.ReadRequest{
		Filename: req.Filename,
	}
	if r := req.Range; r != nil {
		newReq.Range = &cacheutil.FileRange{
			Offset: r.Offset,
			Length: r.Length,
		}
	}
	return cacheutil.ReadFile(ctx, m, newReq)
}

func (r *ref) ReadDir(ctx context.Context, req client.ReadDirRequest) ([]*fstypes.Stat, error) {
	m, err := r.getMountable(ctx)
	if err != nil {
		return nil, err
	}
	newReq := cacheutil.ReadDirRequest{
		Path:           req.Path,
		IncludePattern: req.IncludePattern,
	}
	return cacheutil.ReadDir(ctx, m, newReq)
}

func (r *ref) StatFile(ctx context.Context, req client.StatRequest) (*fstypes.Stat, error) {
	m, err := r.getMountable(ctx)
	if err != nil {
		return nil, err
	}
	return cacheutil.StatFile(ctx, m, req.Path)
}

func (r *ref) getMountable(ctx context.Context) (snapshot.Mountable, error) {
	rr, err := r.ResultProxy.Result(ctx)
	if err != nil {
		return nil, err
	}
	ref, ok := rr.Sys().(*worker.WorkerRef)
	if !ok {
		return nil, errors.Errorf("invalid ref: %T", rr.Sys())
	}
	return ref.ImmutableRef.Mount(ctx, true, r.session)
}
