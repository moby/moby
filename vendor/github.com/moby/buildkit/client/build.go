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
		gwClient := c.gatewayClientForBuild(ref)
		g, err := grpcclient.New(ctx, feOpts, s.ID(), product, gwClient, gworkers)
		if err != nil {
			return err
		}

		gwClient.caps = g.BuildOpts().Caps

		if err := g.Run(ctx, buildFunc); err != nil {
			return errors.Wrap(err, "failed to run Build function")
		}
		return nil
	}

	return c.solve(ctx, nil, cb, opt, statusChan)
}

func (c *Client) gatewayClientForBuild(buildid string) *gatewayClientForBuild {
	g := gatewayapi.NewLLBBridgeClient(c.conn)
	return &gatewayClientForBuild{
		gateway: g,
		buildID: buildid,
	}
}

type gatewayClientForBuild struct {
	gateway gatewayapi.LLBBridgeClient
	buildID string
	caps    apicaps.CapSet
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
	if err := g.caps.Supports(gatewayapi.CapReadDir); err != nil {
		return nil, err
	}
	ctx = buildid.AppendToOutgoingContext(ctx, g.buildID)
	return g.gateway.ReadDir(ctx, in, opts...)
}

func (g *gatewayClientForBuild) StatFile(ctx context.Context, in *gatewayapi.StatFileRequest, opts ...grpc.CallOption) (*gatewayapi.StatFileResponse, error) {
	if err := g.caps.Supports(gatewayapi.CapStatFile); err != nil {
		return nil, err
	}
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

func (g *gatewayClientForBuild) Inputs(ctx context.Context, in *gatewayapi.InputsRequest, opts ...grpc.CallOption) (*gatewayapi.InputsResponse, error) {
	if err := g.caps.Supports(gatewayapi.CapFrontendInputs); err != nil {
		return nil, err
	}
	ctx = buildid.AppendToOutgoingContext(ctx, g.buildID)
	return g.gateway.Inputs(ctx, in, opts...)
}

func (g *gatewayClientForBuild) NewContainer(ctx context.Context, in *gatewayapi.NewContainerRequest, opts ...grpc.CallOption) (*gatewayapi.NewContainerResponse, error) {
	if err := g.caps.Supports(gatewayapi.CapGatewayExec); err != nil {
		return nil, err
	}
	ctx = buildid.AppendToOutgoingContext(ctx, g.buildID)
	return g.gateway.NewContainer(ctx, in, opts...)
}

func (g *gatewayClientForBuild) ReleaseContainer(ctx context.Context, in *gatewayapi.ReleaseContainerRequest, opts ...grpc.CallOption) (*gatewayapi.ReleaseContainerResponse, error) {
	if err := g.caps.Supports(gatewayapi.CapGatewayExec); err != nil {
		return nil, err
	}
	ctx = buildid.AppendToOutgoingContext(ctx, g.buildID)
	return g.gateway.ReleaseContainer(ctx, in, opts...)
}

func (g *gatewayClientForBuild) ExecProcess(ctx context.Context, opts ...grpc.CallOption) (gatewayapi.LLBBridge_ExecProcessClient, error) {
	if err := g.caps.Supports(gatewayapi.CapGatewayExec); err != nil {
		return nil, err
	}
	ctx = buildid.AppendToOutgoingContext(ctx, g.buildID)
	return g.gateway.ExecProcess(ctx, opts...)
}
