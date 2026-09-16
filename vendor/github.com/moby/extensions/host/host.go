// Package host runs extensions and resolves their point providers for a host
// process such as the Moby daemon.
package host

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	servicev0 "github.com/moby/extensions/extpoints/service/v0"
	"github.com/moby/extensions/internal/broker"
	"github.com/moby/extensions/internal/launcher"
	"github.com/moby/extensions/serverpoint"
	"github.com/moby/extensions/servicegrpc"
	"google.golang.org/grpc"
)

type pointPolicyAction uint8

const (
	pointPolicyActionUnspecified pointPolicyAction = iota
	pointPolicyActionAllow
	pointPolicyActionDrop
	pointPolicyActionReject
)

var (
	errPolicyRejected           = errors.New("point policy rejected the requested use")
	errInvalidPointPolicyResult = errors.New("point policy returned an invalid or unspecified result")
)

// PointPolicyResult is an opaque provider-policy decision.
// Its zero value rejects the requested use as an invalid or unspecified
// decision.
type PointPolicyResult struct {
	action pointPolicyAction
	cause  error
}

// Allow returns a decision that keeps an ordinary provider or publishes an
// offered set.
func Allow() PointPolicyResult {
	return PointPolicyResult{action: pointPolicyActionAllow}
}

// Drop returns a decision that silently omits an ordinary provider or leaves an
// offered set private.
func Drop() PointPolicyResult {
	return PointPolicyResult{action: pointPolicyActionDrop}
}

// Reject returns a decision that fails Host construction with cause.
// A nil cause produces a generic policy-rejection error.
func Reject(cause error) PointPolicyResult {
	return PointPolicyResult{action: pointPolicyActionReject, cause: cause}
}

func (result PointPolicyResult) resolve() (pointPolicyAction, error) {
	switch result.action {
	case pointPolicyActionAllow, pointPolicyActionDrop:
		if result.cause != nil {
			return pointPolicyActionReject, errInvalidPointPolicyResult
		}
		return result.action, nil
	case pointPolicyActionReject:
		if result.cause == nil {
			return pointPolicyActionReject, errPolicyRejected
		}
		return pointPolicyActionReject, result.cause
	default:
		return pointPolicyActionReject, errInvalidPointPolicyResult
	}
}

// PointPolicy decides whether an extension identity may provide a Point.
// A decision on servicev0.Point.ID() controls publication of the complete
// offered set.
// A nil policy allows ordinary providers and drops publication.
type PointPolicy interface {
	Decide(identity extensions.ExtensionIdentity, point extensions.PointID) PointPolicyResult
}

// PointPolicyFunc adapts a function to [PointPolicy].
type PointPolicyFunc func(identity extensions.ExtensionIdentity, point extensions.PointID) PointPolicyResult

// Decide calls f.
// A nil function rejects the requested Point use with a generic cause.
func (f PointPolicyFunc) Decide(identity extensions.ExtensionIdentity, point extensions.PointID) PointPolicyResult {
	if f == nil {
		return Reject(nil)
	}
	return f(identity, point)
}

// Option configures a [Host].
type Option interface {
	apply(*options)
}

type optionFunc func(*options)

func (f optionFunc) apply(options *options) {
	f(options)
}

type options struct {
	runtimeDir          string
	extensions          []extensions.Extension
	dirs                []string
	clientProviders     []clientpoint.Registration
	providerPolicy      PointPolicy
	pointServers        []serverpoint.Registration
	reservedServices    []string
	extensionConfig     map[extensions.ExtensionID]extensions.Config
	dependencyProviders []serverpoint.Registration
}

// WithRuntimeDir sets the directory where extension sockets are created.
func WithRuntimeDir(dir string) Option {
	return optionFunc(func(options *options) {
		options.runtimeDir = dir
	})
}

// WithExtensions adds in-process extensions to register.
func WithExtensions(exts ...extensions.Extension) Option {
	exts = append([]extensions.Extension(nil), exts...)
	return optionFunc(func(options *options) {
		options.extensions = append(options.extensions, exts...)
	})
}

// WithDirs adds directories to scan for out-of-process extension binaries.
func WithDirs(dirs ...string) Option {
	dirs = append([]string(nil), dirs...)
	return optionFunc(func(options *options) {
		options.dirs = append(options.dirs, dirs...)
	})
}

// WithClientProviders adds supported out-of-process points and their client
// wiring.
// Unlisted points are rejected unless the extension offered them only for
// external publication.
func WithClientProviders(providers ...clientpoint.Registration) Option {
	providers = append([]clientpoint.Registration(nil), providers...)
	return optionFunc(func(options *options) {
		options.clientProviders = append(options.clientProviders, providers...)
	})
}

// WithProviderPolicy sets the policy deciding which providers are admitted.
// A decision on servicev0.Point.ID() controls publication.
// A nil policy admits all internally wired providers but drops publication.
func WithProviderPolicy(policy PointPolicy) Option {
	return optionFunc(func(options *options) {
		options.providerPolicy = policy
	})
}

// WithPointServers adds generated adapters available for allowed in-process
// offers.
func WithPointServers(servers ...serverpoint.Registration) Option {
	servers = append([]serverpoint.Registration(nil), servers...)
	return optionFunc(func(options *options) {
		options.pointServers = append(options.pointServers, servers...)
	})
}

// WithReservedServices adds daemon-owned gRPC service names that extensions
// cannot publish.
func WithReservedServices(services ...string) Option {
	services = append([]string(nil), services...)
	return optionFunc(func(options *options) {
		options.reservedServices = append(options.reservedServices, services...)
	})
}

// WithExtensionConfig sets configuration keyed by extension id.
// The outer map is copied when this option is created, while configuration
// values are shared.
func WithExtensionConfig(config map[extensions.ExtensionID]extensions.Config) Option {
	var copied map[extensions.ExtensionID]extensions.Config
	if config != nil {
		copied = make(map[extensions.ExtensionID]extensions.Config, len(config))
		maps.Copy(copied, config)
	}
	return optionFunc(func(options *options) {
		options.extensionConfig = copied
	})
}

// WithDependencyProviders adds points launched extensions may call over the
// callback socket.
func WithDependencyProviders(providers ...serverpoint.Registration) Option {
	providers = append([]serverpoint.Registration(nil), providers...)
	return optionFunc(func(options *options) {
		options.dependencyProviders = append(options.dependencyProviders, providers...)
	})
}

// Host runs extensions and resolves their point providers.
type Host struct {
	broker *broker.Broker
	// conns holds connections to launched extensions for socket proxying.
	conns map[extensions.ExtensionID]grpc.ClientConnInterface
	// loaded owns resources acquired for executable extensions in load order.
	loaded []loadedExtension
	// publishedServices contains only services approved by Host policy.
	publishedServices map[extensions.ExtensionID]map[extensions.PointID][]string
	// inProcessServices are approved and prevalidated for daemon registration.
	inProcessServices []servicegrpc.Service
	// callback serves launched extensions' declared dependencies.
	callback *grpc.Server
}

type loadedExtension struct {
	identity  extensions.ExtensionIdentity
	extension extensions.Extension
	close     func(context.Context) error
}

// hostedExtension is the runtime-neutral declaration and lifecycle surface
// needed to adapt an externally hosted extension to the broker.
type hostedExtension struct {
	identity     extensions.ExtensionIdentity
	dependencies []extensions.Dependency
	conflicts    []extensions.ExtensionID
	points       []extensions.PointID
	offered      []extensions.PointID
	conn         grpc.ClientConnInterface
	initialize   func(context.Context, extensions.Config) error
	shutdown     func(context.Context) error
}

// Conn returns the gRPC connection to a launched extension.
func (h *Host) Conn(extension extensions.ExtensionID) (grpc.ClientConnInterface, bool) {
	conn, ok := h.conns[extension]
	return conn, ok
}

// PublishedServicesForPoint returns process service names approved for external
// publication under point.
func (h *Host) PublishedServicesForPoint(point extensions.PointID) map[extensions.ExtensionID][]string {
	out := make(map[extensions.ExtensionID][]string)
	for id, services := range h.publishedServices {
		if names := services[point]; len(names) > 0 {
			out[id] = append([]string(nil), names...)
		}
	}
	return out
}

// RegisterInProcessServices installs all policy-approved in-process Point
// services on registrar.
func (h *Host) RegisterInProcessServices(registrar grpc.ServiceRegistrar) {
	for _, service := range h.inProcessServices {
		service.Register(registrar)
	}
}

// New registers, launches, and initializes the configured extensions. It tears
// down anything started when an error occurs; loading is all-or-nothing.
func New(ctx context.Context, optionList ...Option) (_ *Host, retErr error) {
	var opts options
	for _, option := range optionList {
		if option == nil {
			return nil, errors.New("nil host option")
		}
		option.apply(&opts)
	}

	providers, err := clientProviderMap(opts.clientProviders)
	if err != nil {
		return nil, err
	}
	pointServers, err := serverPointMap(opts.pointServers)
	if err != nil {
		return nil, err
	}
	reservedServices := make(map[string]bool, len(opts.reservedServices))
	for _, service := range opts.reservedServices {
		if service == "" {
			return nil, errors.New("reserved gRPC service name is empty")
		}
		reservedServices[service] = true
	}
	policy := opts.providerPolicy
	b := broker.New()
	conns := make(map[extensions.ExtensionID]grpc.ClientConnInterface)
	var loaded []loadedExtension
	publishedServices := make(map[extensions.ExtensionID]map[extensions.PointID][]string)
	publishedOwners := make(map[string]extensions.ExtensionID)
	var inProcessServices []servicegrpc.Service
	var callback *grpc.Server
	// The broker only shuts down initialized extensions, so construction failures
	// also explicitly close loaded resources.
	defer func() {
		if retErr != nil {
			_ = b.Shutdown(context.Background())
			closeLoaded(context.Background(), loaded)
			if callback != nil {
				callback.Stop()
			}
		}
	}()

	// Put the callback path in each handshake before starting the server.
	callbackEndpoint := ""
	if len(opts.dependencyProviders) > 0 {
		callbackEndpoint = filepath.Join(opts.runtimeDir, "callback.sock")
	}

	for _, ext := range opts.extensions {
		decl := ext.Declaration()
		identity := extensions.ExtensionIdentity{
			ID:     decl.ID,
			Origin: extensions.ExtensionOrigin{Kind: extensions.ExtensionOriginBuiltin},
		}
		if err := validateHostIdentity(identity, decl); err != nil {
			return nil, err
		}
		admittedProviders, err := admitProviders(identity, decl.Providers, policy)
		if err != nil {
			return nil, err
		}
		services, err := collectInProcessPublications(identity, ext, policy, pointServers, publishedServices, publishedOwners, reservedServices)
		if err != nil {
			return nil, err
		}
		inProcessServices = append(inProcessServices, services...)
		decl.Providers = admittedProviders
		if err := b.Register(identity, extensions.New(decl)); err != nil {
			return nil, err
		}
	}
	l := launcher.Launcher{
		RuntimeDir:       opts.runtimeDir,
		ExtensionConfig:  opts.extensionConfig,
		CallbackEndpoint: callbackEndpoint,
	}
	for _, dir := range opts.dirs {
		bins, err := launcher.Binaries(ctx, dir)
		if err != nil {
			return nil, err
		}
		for _, bin := range bins {
			loadedExt, started, err := loadProcess(ctx, l, bin, providers)
			if err != nil {
				return nil, err
			}
			loaded = append(loaded, loadedExt)
			identity := loadedExt.identity
			decl := loadedExt.extension.Declaration()
			if err := validateHostIdentity(identity, decl); err != nil {
				return nil, err
			}
			admittedProviders, err := admitProviders(identity, decl.Providers, policy)
			if err != nil {
				return nil, err
			}
			if err := approveProcessPublications(identity, started, policy, publishedServices, publishedOwners, reservedServices); err != nil {
				return nil, err
			}
			decl.Providers = admittedProviders
			if err := b.Register(identity, extensions.New(decl)); err != nil {
				return nil, err
			}
			conns[identity.ID] = started.Conn
		}
	}

	// Check single-provider constraints across all registered extensions.
	for _, reg := range opts.clientProviders {
		if !reg.Single {
			continue
		}
		point := reg.Point
		if providers := extensions.EffectiveProviders(b.Providers(point)); len(providers) > 1 {
			ids := make([]string, len(providers))
			for i, p := range providers {
				ids[i] = string(p.Identity.ID)
			}
			return nil, fmt.Errorf("point %q admits a single provider, but extensions %s all provide it", point, strings.Join(ids, ", "))
		}
	}

	// Serve dependencies before initialization so dependency callbacks reach an
	// initialized provider.
	if callbackEndpoint != "" {
		callback, err = serveCallback(callbackEndpoint, opts.dependencyProviders, b)
		if err != nil {
			return nil, err
		}
	}

	if err := b.Init(ctx, opts.extensionConfig); err != nil {
		return nil, err
	}
	return &Host{broker: b, conns: conns, loaded: loaded, publishedServices: publishedServices, inProcessServices: inProcessServices, callback: callback}, nil
}

func validateHostIdentity(identity extensions.ExtensionIdentity, decl extensions.Declaration) error {
	if err := extensions.ValidateExtensionIdentity(identity); err != nil {
		return err
	}
	if identity.ID != decl.ID {
		return fmt.Errorf("extension identity id %q does not match declared id %q", identity.ID, decl.ID)
	}
	return nil
}

func admitProviders(identity extensions.ExtensionIdentity, providers []extensions.Provider, policy PointPolicy) ([]extensions.Provider, error) {
	admitted := make([]extensions.Provider, 0, len(providers))
	for _, provider := range providers {
		if extensions.IsMetadataPoint(provider.Point) {
			admitted = append(admitted, provider)
			continue
		}
		result := Allow()
		if policy != nil {
			result = policy.Decide(identity, provider.Point)
		}
		action, cause := result.resolve()
		switch action {
		case pointPolicyActionAllow:
			admitted = append(admitted, provider)
		case pointPolicyActionDrop:
			continue
		default:
			return nil, policyRejectionError("admit provider", identity, provider.Point, cause)
		}
	}
	return admitted, nil
}

func approveProcessPublications(identity extensions.ExtensionIdentity, started *launcher.Launched, policy PointPolicy, published map[extensions.ExtensionID]map[extensions.PointID][]string, owners map[string]extensions.ExtensionID, reserved map[string]bool) error {
	if len(started.OfferedPoints) == 0 {
		return nil
	}
	result := Drop()
	if policy != nil {
		result = policy.Decide(identity, servicev0.Point.ID())
	}
	action, cause := result.resolve()
	switch action {
	case pointPolicyActionDrop:
		return nil
	case pointPolicyActionReject:
		return policyRejectionError("publish offered points", identity, servicev0.Point.ID(), cause)
	}
	for _, point := range started.OfferedPoints {
		names := started.ProviderServices[point]
		if len(names) == 0 {
			return fmt.Errorf("extension %q offered point %q without a gRPC service", identity.ID, point)
		}
		for _, service := range names {
			if reserved[service] {
				return fmt.Errorf("extension %q cannot publish reserved gRPC service %q", identity.ID, service)
			}
			if owner, exists := owners[service]; exists {
				return fmt.Errorf("extensions %q and %q both publish gRPC service %q", owner, identity.ID, service)
			}
			owners[service] = identity.ID
		}
		if published[identity.ID] == nil {
			published[identity.ID] = make(map[extensions.PointID][]string)
		}
		published[identity.ID][point] = append([]string(nil), names...)
	}
	return nil
}

func collectInProcessPublications(identity extensions.ExtensionIdentity, ext extensions.Extension, policy PointPolicy, servers map[extensions.PointID]serverpoint.Registration, published map[extensions.ExtensionID]map[extensions.PointID][]string, owners map[string]extensions.ExtensionID, reserved map[string]bool) ([]servicegrpc.Service, error) {
	decl := ext.Declaration()
	providers := make(map[extensions.PointID]any, len(decl.Providers))
	for _, provider := range decl.Providers {
		if _, exists := providers[provider.Point]; exists {
			return nil, fmt.Errorf("extension %q provides point %q more than once", decl.ID, provider.Point)
		}
		providers[provider.Point] = provider.Impl
	}

	var offered []extensions.PointID
	seen := make(map[extensions.PointID]bool)
	for _, provider := range decl.Providers {
		if provider.Point != servicev0.Point.ID() {
			continue
		}
		metadata, ok := provider.Impl.(servicev0.Provider)
		if !ok {
			return nil, fmt.Errorf("extension %q: point %q has incompatible offer metadata", decl.ID, provider.Point)
		}
		for _, point := range metadata.OfferedPoints() {
			if err := extensions.ValidatePointID(point); err != nil {
				return nil, fmt.Errorf("extension %q: invalid offered point: %w", decl.ID, err)
			}
			if point == servicev0.Point.ID() {
				return nil, fmt.Errorf("extension %q: publication metadata point %q cannot offer itself", decl.ID, point)
			}
			if seen[point] {
				return nil, fmt.Errorf("extension %q: point %q is offered more than once", decl.ID, point)
			}
			seen[point] = true
			_, implemented := providers[point]
			if !implemented {
				return nil, fmt.Errorf("extension %q: offered point %q is not implemented", decl.ID, point)
			}
			offered = append(offered, point)
		}
	}
	if len(offered) == 0 {
		return nil, nil
	}
	result := Drop()
	if policy != nil {
		result = policy.Decide(identity, servicev0.Point.ID())
	}
	action, cause := result.resolve()
	switch action {
	case pointPolicyActionDrop:
		return nil, nil
	case pointPolicyActionReject:
		return nil, policyRejectionError("publish offered points", identity, servicev0.Point.ID(), cause)
	}

	var services []servicegrpc.Service
	for _, point := range offered {
		impl := providers[point]
		registration, ok := servers[point]
		if !ok {
			return nil, fmt.Errorf("extension %q: allowed in-process offer for point %q has no server registration", decl.ID, point)
		}
		service, err := servicegrpc.Adapt(registration, impl)
		if err != nil {
			return nil, fmt.Errorf("extension %q: publish point %q: %w", decl.ID, point, err)
		}
		if reserved[service.Name] {
			return nil, fmt.Errorf("extension %q cannot publish reserved gRPC service %q", decl.ID, service.Name)
		}
		if owner, exists := owners[service.Name]; exists {
			return nil, fmt.Errorf("extensions %q and %q both publish gRPC service %q", owner, decl.ID, service.Name)
		}
		owners[service.Name] = decl.ID
		services = append(services, service)
		if published[decl.ID] == nil {
			published[decl.ID] = make(map[extensions.PointID][]string)
		}
		published[decl.ID][point] = []string{service.Name}
	}
	return services, nil
}

func policyRejectionError(operation string, identity extensions.ExtensionIdentity, point extensions.PointID, cause error) error {
	if executable := identity.Origin.Executable; executable != nil {
		return fmt.Errorf("%s for extension %q with origin %q at %q for point %q: %w", operation, identity.ID, identity.Origin.Kind, executable.Path, point, cause)
	}
	return fmt.Errorf("%s for extension %q with origin %q for point %q: %w", operation, identity.ID, identity.Origin.Kind, point, cause)
}

// serveCallback starts the server for launched extensions' declared
// dependencies. It rejects ambiguous points because the callback serves one
// provider per point.
func serveCallback(endpoint string, deps []serverpoint.Registration, b *broker.Broker) (*grpc.Server, error) {
	srv := grpc.NewServer()
	if err := registerDependencyProviders(srv, deps, b); err != nil {
		return nil, err
	}
	if err := os.Remove(endpoint); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale dependency callback socket: %w", err)
	}
	lis, err := net.Listen("unix", endpoint)
	if err != nil {
		return nil, fmt.Errorf("listen on dependency callback socket: %w", err)
	}
	go func() { _ = srv.Serve(lis) }()
	return srv, nil
}

// registerDependencyProviders applies the callback's single-provider policy to
// any gRPC service registrar.
func registerDependencyProviders(registrar grpc.ServiceRegistrar, deps []serverpoint.Registration, b *broker.Broker) error {
	for _, dep := range deps {
		providers := extensions.EffectiveProviders(b.Providers(dep.Point))
		switch len(providers) {
		case 0:
			continue
		case 1:
			dep.Register(registrar, providers[0].Impl)
		default:
			return fmt.Errorf("dependency point %q offered on the callback has %d providers; exactly one is required", dep.Point, len(providers))
		}
	}
	return nil
}

// Provider returns one provider for point implemented by extension.
func (h *Host) Provider(point extensions.PointID, extension extensions.ExtensionID) (any, error) {
	return h.broker.Provider(point, extension)
}

// Providers returns all providers for point.
func (h *Host) Providers(point extensions.PointID) []extensions.ResolvedProvider {
	return h.broker.Providers(point)
}

// Shutdown stops extensions in reverse dependency order, then closes every
// loaded resource and the dependency callback server.
func (h *Host) Shutdown(ctx context.Context) error {
	err := h.broker.Shutdown(ctx)
	err = errors.Join(err, closeLoadedErr(ctx, h.loaded))
	if h.callback != nil {
		h.callback.Stop()
	}
	return err
}

// closeLoaded is best-effort teardown for failed host construction.
func closeLoaded(ctx context.Context, loaded []loadedExtension) {
	_ = closeLoadedErr(ctx, loaded)
}

// closeLoadedErr closes resources in reverse load order.
func closeLoadedErr(ctx context.Context, loaded []loadedExtension) error {
	var errs []error
	for _, l := range slices.Backward(loaded) {
		errs = append(errs, l.close(ctx))
	}
	return errors.Join(errs...)
}

// loadProcess launches and adapts one out-of-process extension.
// The launched process remains guarded until the loaded resource is returned to
// its owner.
func loadProcess(ctx context.Context, l launcher.Launcher, bin string, providers map[extensions.PointID]clientpoint.Provider) (loadedExtension, *launcher.Launched, error) {
	launched, err := l.Launch(ctx, bin)
	if err != nil {
		return loadedExtension{}, nil, err
	}

	owned := true
	defer func() {
		if owned {
			_ = launched.Close(context.Background())
		}
	}()

	hosted := hostedExtensionFromLaunched(launched)
	ext, err := extensionFromHosted(hosted, providers)
	if err != nil {
		return loadedExtension{}, nil, err
	}
	loaded := loadedExtension{identity: hosted.identity, extension: ext, close: launched.Close}
	owned = false
	return loaded, launched, nil
}

// clientProviderMap indexes registrations by point id.
func clientProviderMap(regs []clientpoint.Registration) (map[extensions.PointID]clientpoint.Provider, error) {
	m := make(map[extensions.PointID]clientpoint.Provider, len(regs))
	for _, r := range regs {
		if _, ok := m[r.Point]; ok {
			return nil, fmt.Errorf("duplicate client provider for point %q", r.Point)
		}
		m[r.Point] = r.Provider
	}
	return m, nil
}

func serverPointMap(regs []serverpoint.Registration) (map[extensions.PointID]serverpoint.Registration, error) {
	servers := make(map[extensions.PointID]serverpoint.Registration, len(regs))
	for _, registration := range regs {
		if registration.Point == "" || registration.Register == nil {
			return nil, errors.New("incomplete in-process server registration")
		}
		if _, exists := servers[registration.Point]; exists {
			return nil, fmt.Errorf("duplicate in-process server registration for point %q", registration.Point)
		}
		servers[registration.Point] = registration
	}
	return servers, nil
}

// hostedExtensionFromLaunched adapts process declaration and lifecycle fields to
// the runtime-neutral hosted extension contract.
func hostedExtensionFromLaunched(launched *launcher.Launched) hostedExtension {
	points := make([]extensions.PointID, 0, len(launched.Points))
	for _, point := range launched.Points {
		points = append(points, point.ID)
	}
	return hostedExtension{
		identity: extensions.ExtensionIdentity{
			ID: launched.ID,
			Origin: extensions.ExtensionOrigin{
				Kind:       extensions.ExtensionOriginExecutable,
				Executable: &extensions.ExecutableOrigin{Path: launched.Path},
			},
		},
		dependencies: launched.Dependencies,
		conflicts:    launched.Conflicts,
		points:       points,
		offered:      append([]extensions.PointID(nil), launched.OfferedPoints...),
		conn:         launched.Conn,
		// Configuration already arrived in the process launch handshake, so the
		// broker's configuration is intentionally ignored.
		initialize: func(ctx context.Context, _ extensions.Config) error {
			return launched.Initialize(ctx)
		},
		shutdown: launched.Close,
	}
}

// extensionFromHosted builds a declaration and client providers from one
// runtime-neutral hosted extension.
func extensionFromHosted(hosted hostedExtension, providers map[extensions.PointID]clientpoint.Provider) (extensions.Extension, error) {
	decl := extensions.Declaration{
		ID:           hosted.identity.ID,
		Dependencies: hosted.dependencies,
		Conflicts:    hosted.conflicts,
		Init: func(ctx context.Context, config extensions.Config, _ extensions.Resolver) error {
			return hosted.initialize(ctx, config)
		},
		// Let the broker stop the extension in reverse dependency order, keeping
		// its dependencies alive until its Shutdown hook has run.
		Shutdown: hosted.shutdown,
	}
	offered := make(map[extensions.PointID]bool, len(hosted.offered))
	for _, point := range hosted.offered {
		offered[point] = true
	}
	for _, point := range hosted.points {
		if point == servicev0.Point.ID() {
			// Publication metadata has no in-daemon provider or client adapter.
			continue
		}
		build, ok := providers[point]
		if !ok {
			if offered[point] {
				// An offered-only Point is proxied externally and has no in-daemon caller.
				continue
			}
			// The daemon cannot call an unlisted point.
			return nil, fmt.Errorf("extension %q declares unsupported point %q", hosted.identity.ID, point)
		}
		provider := build(hosted.conn)
		decl.Providers = append(decl.Providers, provider)
	}
	return extensions.New(decl), nil
}
