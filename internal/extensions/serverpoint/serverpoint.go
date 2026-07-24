// Package serverpoint describes how an out-of-process extension serves a point.
// A point's generated wiring exposes a [Registration]; an SDK server uses a set
// of them to register the gRPC service for each provider an extension declares.
// It is the server-side mirror of clientpoint, and like it is deliberately tiny
// -- depending only on extensions and grpc -- so a point's generated code can
// reference it without pulling in the SDK server.
package serverpoint

import (
	"github.com/moby/moby/v2/internal/extensions"
	"google.golang.org/grpc"
)

// Register registers the gRPC service that serves a point on r, wrapping the
// provider implementation impl (the point's Go interface, passed as any).
type Register func(r grpc.ServiceRegistrar, impl any)

// Registration pairs a point id with its server registration.
type Registration struct {
	Point    extensions.PointID
	Register Register
}
