// Package clientpoint defines the host-side registration for generated
// out-of-process point providers. It has no dependency on the host runtime so
// generated code can reference it.
package clientpoint

import (
	"github.com/moby/extensions"
	"google.golang.org/grpc"
)

// Provider builds an in-broker provider from a gRPC connection.
type Provider func(grpc.ClientConnInterface) extensions.Provider

// Registration pairs a point id with its client provider. Single records the
// contract's one-provider cardinality.
type Registration struct {
	Point    extensions.PointID
	Provider Provider
	Single   bool
}
