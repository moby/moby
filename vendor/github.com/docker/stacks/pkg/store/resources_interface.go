package store

import (
	"context"

	swarmapi "github.com/docker/swarmkit/api"
	"google.golang.org/grpc"
)

// ResourcesClient is a subset of swarmkit's ControlClient interface for operating on
// Resources and Extensions
type ResourcesClient interface {
	CreateExtension(ctx context.Context, in *swarmapi.CreateExtensionRequest, opts ...grpc.CallOption) (*swarmapi.CreateExtensionResponse, error)
	GetResource(ctx context.Context, in *swarmapi.GetResourceRequest, opts ...grpc.CallOption) (*swarmapi.GetResourceResponse, error)
	UpdateResource(ctx context.Context, in *swarmapi.UpdateResourceRequest, opts ...grpc.CallOption) (*swarmapi.UpdateResourceResponse, error)
	ListResources(ctx context.Context, in *swarmapi.ListResourcesRequest, opts ...grpc.CallOption) (*swarmapi.ListResourcesResponse, error)
	CreateResource(ctx context.Context, in *swarmapi.CreateResourceRequest, opts ...grpc.CallOption) (*swarmapi.CreateResourceResponse, error)
	RemoveResource(ctx context.Context, in *swarmapi.RemoveResourceRequest, opts ...grpc.CallOption) (*swarmapi.RemoveResourceResponse, error)
}
