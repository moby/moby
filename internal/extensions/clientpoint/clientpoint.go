// Package clientpoint describes how a host builds an out-of-process provider
// for a point. A point's generated wiring exposes a [Registration]; a host
// turns a set of them into its client-provider lookup. It is deliberately tiny
// -- it depends only on extensions and grpc -- so a point's generated code can
// reference it without pulling in the host runtime (the broker and launcher).
package clientpoint

import (
	"github.com/moby/moby/v2/internal/extensions"
	"google.golang.org/grpc"
)

// Provider builds an in-broker provider for a point from a gRPC connection to
// the extension serving it.
type Provider func(grpc.ClientConnInterface) extensions.Provider

// Registration pairs a point id with its client provider.
type Registration struct {
	Point    extensions.PointID
	Provider Provider
}
