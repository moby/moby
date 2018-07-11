package forwarder

import (
	"context"

	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/frontend/gateway/client"
)

func NewGatewayForwarder(w frontend.WorkerInfos, f client.BuildFunc) frontend.Frontend {
	return &GatewayForwarder{
		workers: w,
		f:       f,
	}
}

type GatewayForwarder struct {
	workers frontend.WorkerInfos
	f       client.BuildFunc
}

func (gf *GatewayForwarder) Solve(ctx context.Context, llbBridge frontend.FrontendLLBBridge, opts map[string]string) (retRes *frontend.Result, retErr error) {
	c, err := llbBridgeToGatewayClient(ctx, llbBridge, opts, gf.workers.WorkerInfos())
	if err != nil {
		return nil, err
	}

	defer func() {
		c.discard(retErr)
	}()

	res, err := gf.f(ctx, c)
	if err != nil {
		return nil, err
	}

	return c.toFrontendResult(res)
}
