package client

import (
	"context"

	"github.com/moby/buildkit/client/buildid"
	gateway "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	gatewayapi "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/apicaps"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

func (c *Client) Build(ctx context.Context, opt SolveOpt, product string, buildFunc gateway.BuildFunc, statusChan chan *SolveStatus) (*SolveResponse, error) {
	defer func() {
		if statusChan != nil {
			close(statusChan)
		}
	}()

	if opt.Frontend != "" {
		return nil, errors.New("invalid SolveOpt, Build interface cannot use Frontend")
	}

	if product == "" {
		product = apicaps.ExportedProduct
	}

	feOpts := opt.FrontendAttrs
	opt.FrontendAttrs = nil

	workers, err := c.ListWorkers(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "listing workers for Build")
	}
	var gworkers []gateway.WorkerInfo
	for _, w := range workers {
		gworkers = append(gworkers, gateway.WorkerInfo{
			ID:        w.ID,
			Labels:    w.Labels,
			Platforms: w.Platforms,
		})
	}

	cb := func(ref string, s *session.Session) error {
		g, err := grpcclient.New(ctx, feOpts, s.ID(), product, c.gatewayClientForBuild(ref), gworkers)
		if err != nil {
			return err
		}

		if err := g.Run(ctx, buildFunc); err != nil {
			return errors.Wrap(err, "failed to run Build function")
		}
		return nil
	}

	return c.solve(ctx, nil, cb, opt, statusChan)
}

func (c *Client) gatewayClientForBuild(buildid string) gatewayapi.LLBBridgeClient {
	g := gatewayapi.NewLLBBridgeClient(c.conn)
	return &gatewayClientForBuild{g, buildid}
}

type gatewayClientForBuild struct {
	gateway gatewayapi.LLBBridgeClient
	buildID string
}

func (g *gatewayClientForBuild) ResolveImageConfig(ctx context.Context, in *gatewayapi.ResolveImageConfigRequest, opts ...grpc.CallOption) (*gatewayapi.ResolveImageConfigResponse, error) {
	ctx = buildid.AppendToOutgoingContext(ctx, g.buildID)
	return g.gateway.ResolveImageConfig(ctx, in, opts...)
}

func (g *gatewayClientForBuild) Solve(ctx context.Context, in *gatewayapi.SolveRequest, opts ...grpc.CallOption) (*gatewayapi.SolveResponse, error) {
	ctx = buildid.AppendToOutgoingContext(ctx, g.buildID)
	return g.gateway.Solve(ctx, in, opts...)
}

func (g *gatewayClientForBuild) ReadFile(ctx context.Context, in *gatewayapi.ReadFileRequest, opts ...grpc.CallOption) (*gatewayapi.ReadFileResponse, error) {
	ctx = buildid.AppendToOutgoingContext(ctx, g.buildID)
	return g.gateway.ReadFile(ctx, in, opts...)
}

func (g *gatewayClientForBuild) ReadDir(ctx context.Context, in *gatewayapi.ReadDirRequest, opts ...grpc.CallOption) (*gatewayapi.ReadDirResponse, error) {
	ctx = buildid.AppendToOutgoingContext(ctx, g.buildID)
	return g.gateway.ReadDir(ctx, in, opts...)
}

func (g *gatewayClientForBuild) StatFile(ctx context.Context, in *gatewayapi.StatFileRequest, opts ...grpc.CallOption) (*gatewayapi.StatFileResponse, error) {
	ctx = buildid.AppendToOutgoingContext(ctx, g.buildID)
	return g.gateway.StatFile(ctx, in, opts...)
}

func (g *gatewayClientForBuild) Ping(ctx context.Context, in *gatewayapi.PingRequest, opts ...grpc.CallOption) (*gatewayapi.PongResponse, error) {
	ctx = buildid.AppendToOutgoingContext(ctx, g.buildID)
	return g.gateway.Ping(ctx, in, opts...)
}

func (g *gatewayClientForBuild) Return(ctx context.Context, in *gatewayapi.ReturnRequest, opts ...grpc.CallOption) (*gatewayapi.ReturnResponse, error) {
	ctx = buildid.AppendToOutgoingContext(ctx, g.buildID)
	return g.gateway.Return(ctx, in, opts...)
}
