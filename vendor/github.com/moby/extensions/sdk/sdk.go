package sdk

import (
	"context"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/extensions/sdk/sdkapi"
	"google.golang.org/grpc"
)

// Server serves one extension from a separate process. Register adds the
// extension, Depends declares callback dependencies, and Listen serves it.
// Init runs when the daemon calls Initialize in dependency order.
type Server struct {
	declaration *sdkapi.Declaration
	grpc        *grpc.Server
	init        func(context.Context, extensions.Config, extensions.Resolver) error
	shutdown    func(context.Context) error
	registered  bool
	initialized bool

	// depends maps each dependency point to the client adapter that reaches its
	// provider over the callback connection.
	depends map[extensions.PointID]clientpoint.Provider

	// Set from the handshake and held for the deferred Initialize.
	config           extensions.Config
	callbackEndpoint string
	initCtx          context.Context
	callbackConn     *grpc.ClientConn
}

// NewServer returns an empty server. Add an extension with Register.
func NewServer() *Server {
	return &Server{
		declaration: &sdkapi.Declaration{},
		grpc:        grpc.NewServer(),
	}
}
