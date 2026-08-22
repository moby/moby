package sdk

import (
	"context"

	"github.com/moby/extensions/sdk/sdkapi"
)

// runtimeServer implements the extension runtime contract for this server.
type runtimeServer struct {
	s *Server
}

func (rt runtimeServer) Describe(context.Context, *sdkapi.DescribeRequest) (*sdkapi.DescribeResponse, error) {
	return &sdkapi.DescribeResponse{Declaration: rt.s.declaration}, nil
}

func (rt runtimeServer) Initialize(context.Context, *sdkapi.InitializeRequest) (*sdkapi.InitializeResponse, error) {
	if err := rt.s.initialize(); err != nil {
		return nil, err
	}
	return &sdkapi.InitializeResponse{}, nil
}
