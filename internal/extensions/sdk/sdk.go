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
	"github.com/moby/moby/v2/internal/extensions/sdk/sdkapi"
	sdkapipb "github.com/moby/moby/v2/internal/extensions/sdk/sdkapi/protogen"
	"github.com/moby/moby/v2/internal/extensions/serverpoint"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

// Register adds ext and serves each provider with its matching serverpoint. The
// registrations must cover every point ext provides. The SDK records service
// names by provider point; the daemon decides which services are public.
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
	s.declaration.ID = string(decl.ID)
	for _, provider := range decl.Providers {
		register, ok := byPoint[provider.Point]
		if !ok {
			return fmt.Errorf("extension %q: no server registration for point %q", decl.ID, provider.Point)
		}
		rec := &recordingRegistrar{target: s.grpc}
		register(rec, provider.Impl)
		s.declaration.Providers = append(s.declaration.Providers, sdkapi.PointDeclaration{ID: string(provider.Point)})
		s.declaration.ProviderServices = append(s.declaration.ProviderServices, sdkapi.ProviderServices{
			Point:    string(provider.Point),
			Services: rec.names,
		})
	}
	for _, dep := range decl.Dependencies {
		s.declaration.Dependencies = append(s.declaration.Dependencies, sdkapi.Dependency{
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

// recordingRegistrar records service names while forwarding registrations.
type recordingRegistrar struct {
	target grpc.ServiceRegistrar
	names  []string
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.names = append(r.names, desc.ServiceName)
	r.target.RegisterService(desc, impl)
}

// Depends registers client wiring for points the extension calls through the
// callback channel.
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

	s.config = cfg.Config
	s.callbackEndpoint = cfg.CallbackEndpoint
	s.initCtx = ctx

	defer func() {
		if s.callbackConn != nil {
			_ = s.callbackConn.Close()
		}
		// Shutdown only what initialized, using a context detached from cancellation
		// because Serve usually returns after the daemon signals the process.
		if s.initialized && s.shutdown != nil {
			if err := s.shutdown(context.WithoutCancel(ctx)); err != nil {
				log.G(ctx).WithError(err).Warn("extension shutdown failed")
			}
		}
	}()

	sdkapipb.RegisterServer(s.grpc, runtimeServer{s: s})
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

// initialize runs Init with the handshake config and callback resolver.
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

// resolver returns the callback-backed dependency resolver.
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

// callbackResolver resolves declared dependencies through the daemon callback.
type callbackResolver struct {
	conn    grpc.ClientConnInterface
	clients map[extensions.PointID]clientpoint.Provider
}

// provider builds a caller for the daemon's provider of point. The callback
// serves one provider per point.
func (r *callbackResolver) provider(point extensions.PointID) (any, error) {
	build, ok := r.clients[point]
	if !ok || r.conn == nil {
		return nil, fmt.Errorf("extension has no resolvable dependency for point %q (declare it with Depends)", point)
	}
	return build(r.conn).Impl, nil
}

func (r *callbackResolver) Provider(point extensions.PointID, _ extensions.ExtensionID) (any, error) {
	// Named selection uses the callback's single provider; by-id selection is not
	// available across the process boundary.
	return r.provider(point)
}

func (r *callbackResolver) Providers(point extensions.PointID) []extensions.ResolvedProvider {
	impl, err := r.provider(point)
	if err != nil {
		return nil
	}
	return []extensions.ResolvedProvider{{Impl: impl}}
}

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
