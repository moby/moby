// Package serverpoint defines the SDK-side registration for generated
// out-of-process point providers. It has no dependency on the SDK server so
// generated code can reference it.
package serverpoint

import (
	"github.com/moby/extensions"
	"google.golang.org/grpc"
)

// Register registers a point's gRPC service on r.
type Register func(r grpc.ServiceRegistrar, impl any)

// Registration pairs a point id with its server registration.
type Registration struct {
	Point    extensions.PointID
	Register Register
}
