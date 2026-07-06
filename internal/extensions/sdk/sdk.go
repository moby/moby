package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/internal/extensions"
	"github.com/moby/moby/v2/internal/extensions/clientpoint"
	"github.com/moby/moby/v2/internal/extensions/sdk/sdkpb"
	"github.com/moby/moby/v2/internal/extensions/serverpoint"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Server serves one extension from a separate process. Register adds the
// extension, Depends declares which dependency points it can reach, and Listen
// serves it. It is the out-of-process counterpart to the in-process broker: both
// register an [extensions.Extension] and run its Init and Shutdown around its
// providers.
//
// Init does not run at startup. The daemon connects every extension, then calls
// the Initialize RPC in dependency order (see [sdkpb]), so that when an
// extension initializes its dependencies are already up and reachable over the
// callback channel.
type Server struct {
	declaration *sdkpb.Declaration
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
		declaration: &sdkpb.Declaration{},
		grpc:        grpc.NewServer(),
	}
}

// Register adds ext to the server: it declares the extension's id, providers,
// dependencies, and conflicts, and serves each provider's gRPC service on the
// server using the matching serverpoint. points must cover every point ext
// provides. One server serves one extension.
//
// Serving is uniform: every provider's service is registered the same way, so
// the daemon can reach it on the per-extension socket. Whether a service is also
// published on the daemon's API socket is the daemon's decision, driven by the
// extension's service.grpc declaration, not a serving mode here. The SDK records
// service names per provider point and reports that inventory to the daemon; it
// does not decide which point is public.
func (s *Server) Register(ext extensions.Extension, points ...serverpoint.Registration) error {
	if s.registered {
		return errors.New("server already has an extension")
	}
	decl := ext.Declaration()
	if decl.ID == "" {
		return errors.New("extension id is required")
	}
	byPoint := make(map[extensions.PointID]serverpoint.Register, len(points))
	for _, p := range points {
		byPoint[p.Point] = p.Register
	}
	s.declaration.Id = string(decl.ID)
	for _, provider := range decl.Providers {
		register, ok := byPoint[provider.Point]
		if !ok {
			return fmt.Errorf("extension %q: no server registration for point %q", decl.ID, provider.Point)
		}
		rec := &recordingRegistrar{target: s.grpc}
		register(rec, provider.Impl)
		s.declaration.Providers = append(s.declaration.Providers, &sdkpb.PointDeclaration{Id: string(provider.Point)})
		s.declaration.ProviderServices = append(s.declaration.ProviderServices, &sdkpb.ProviderServices{
			Point:    string(provider.Point),
			Services: rec.names,
		})
	}
	for _, dep := range decl.Dependencies {
		s.declaration.Dependencies = append(s.declaration.Dependencies, &sdkpb.Dependency{
			Point:     string(dep.Point),
			Extension: string(dep.Extension),
			Optional:  dep.Optional,
		})
	}
	for _, id := range decl.Conflicts {
		s.declaration.Conflicts = append(s.declaration.Conflicts, string(id))
	}
	s.init = decl.Init
	s.shutdown = decl.Shutdown
	s.registered = true
	return nil
}

// recordingRegistrar records the names of the services registered on it while
// passing each registration through to target. It lets the SDK serve a
// provider's services on its own server and learn their names in one pass, so it
// can report them to the daemon -- without knowing what the services are.
type recordingRegistrar struct {
	target grpc.ServiceRegistrar
	names  []string
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.names = append(r.names, desc.ServiceName)
	r.target.RegisterService(desc, impl)
}

// Depends registers the client wiring for the points this extension declares a
// dependency on, so the resolver its Init receives can build a caller for each
// over the callback channel. A point contract's generated wiring exposes one as
// ClientPoint; pass one per dependency point the extension will call.
func (s *Server) Depends(regs ...clientpoint.Registration) {
	if s.depends == nil {
		s.depends = make(map[extensions.PointID]clientpoint.Provider, len(regs))
	}
	for _, r := range regs {
		s.depends[r.Point] = r.Provider
	}
}

// Listen reads the startup config from stdin and serves the registered services.
func (s *Server) Listen(ctx context.Context) error {
	return s.ListenWithIO(ctx, os.Stdin, os.Stdout)
}

// ListenWithIO is Listen with explicit streams, for tests.
func (s *Server) ListenWithIO(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	var cfg StartupConfig
	if err := json.NewDecoder(stdin).Decode(&cfg); err != nil {
		return fmt.Errorf("read startup config: %w", err)
	}
	if cfg.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported extension protocol version %d", cfg.ProtocolVersion)
	}
	listener, err := net.Listen("unix", cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("listen on extension socket: %w", err)
	}
	defer listener.Close()

	// Held for the deferred Initialize (see the Extension.Initialize RPC).
	s.config = cfg.Config
	s.callbackEndpoint = cfg.CallbackEndpoint
	s.initCtx = ctx

	defer func() {
		if s.callbackConn != nil {
			_ = s.callbackConn.Close()
		}
		// Only tear down what was initialized. ctx is typically already cancelled
		// by the time Serve returns (the daemon stops the extension by signalling
		// it), so shut down with a context detached from that cancellation.
		if s.initialized && s.shutdown != nil {
			if err := s.shutdown(context.WithoutCancel(ctx)); err != nil {
				log.G(ctx).WithError(err).Warn("extension shutdown failed")
			}
		}
	}()

	sdkpb.RegisterExtensionServer(s.grpc, runtimeServer{s: s})
	if _, err := io.WriteString(stdout, ReadinessAck); err != nil {
		return fmt.Errorf("write readiness ack: %w", err)
	}
	go func() {
		<-ctx.Done()
		s.grpc.GracefulStop()
	}()
	if err := s.grpc.Serve(listener); err != nil {
		log.G(ctx).WithError(err).Debug("extension gRPC server stopped")
	}
	return nil
}

// initialize runs the extension's Init with its config and a resolver over the
// callback channel. It is invoked by the Initialize RPC, in dependency order.
func (s *Server) initialize() error {
	if s.init == nil {
		s.initialized = true
		return nil
	}
	resolver, err := s.resolver()
	if err != nil {
		return err
	}
	if err := s.init(s.initCtx, s.config, resolver); err != nil {
		return fmt.Errorf("initialize extension: %w", err)
	}
	s.initialized = true
	return nil
}

// resolver returns the dependency resolver backed by the callback connection.
// It dials the callback lazily on first init.
func (s *Server) resolver() (extensions.Resolver, error) {
	if s.callbackEndpoint != "" && s.callbackConn == nil {
		conn, err := grpc.NewClient("unix:"+s.callbackEndpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", s.callbackEndpoint)
			}),
		)
		if err != nil {
			return nil, fmt.Errorf("connect to dependency callback: %w", err)
		}
		s.callbackConn = conn
	}
	return &callbackResolver{conn: s.callbackConn, clients: s.depends}, nil
}

// callbackResolver resolves an extension's declared dependencies over the
// callback connection to the daemon, which routes each to the real provider.
type callbackResolver struct {
	conn    grpc.ClientConnInterface
	clients map[extensions.PointID]clientpoint.Provider
}

func (r *callbackResolver) SingleProvider(point extensions.PointID) (any, error) {
	build, ok := r.clients[point]
	if !ok || r.conn == nil {
		return nil, fmt.Errorf("extension has no resolvable dependency for point %q (declare it with Depends)", point)
	}
	return build(r.conn).Impl, nil
}

func (r *callbackResolver) Provider(point extensions.PointID, _ extensions.ExtensionID) (any, error) {
	// Named selection routes to the daemon's single provider for the point;
	// selecting a specific provider by extension id across the boundary is future.
	return r.SingleProvider(point)
}

func (r *callbackResolver) Providers(point extensions.PointID) []extensions.ResolvedProvider {
	impl, err := r.SingleProvider(point)
	if err != nil {
		return nil
	}
	return []extensions.ResolvedProvider{{Impl: impl}}
}

type runtimeServer struct {
	sdkpb.UnimplementedExtensionServer
	s *Server
}

func (rt runtimeServer) Describe(context.Context, *sdkpb.DescribeRequest) (*sdkpb.DescribeResponse, error) {
	return &sdkpb.DescribeResponse{Declaration: rt.s.declaration}, nil
}

func (rt runtimeServer) Initialize(context.Context, *sdkpb.InitializeRequest) (*sdkpb.InitializeResponse, error) {
	if err := rt.s.initialize(); err != nil {
		return nil, err
	}
	return &sdkpb.InitializeResponse{}, nil
}
