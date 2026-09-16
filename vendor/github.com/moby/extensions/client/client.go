// Package client provides typed, lazy clients for extension points.
//
// Generated wire bindings expose a ClientPoint registration containing the
// runtime gRPC adapter factory.
// A handwritten extensions.Point supplies the compile-time result type used by
// Resolve.
// Each Client owns a separate lazy connection derived from an Engine dialer.
// Resolve only constructs a local proxy, so unavailable or unpublished services
// report errors when a resolved method is invoked rather than during Resolve.
//
// A generated greeter binding can be configured and resolved as follows:
//
//	extensions, err := client.New(
//		engine,
//		client.WithGRPCPoint(greeterpb.ClientPoint),
//	)
//	if err != nil {
//		return err
//	}
//	defer extensions.Close()
//
//	greeter, err := client.Resolve(extensions, greeterv0.Point)
//	if err != nil {
//		return err
//	}
//
//	reply, err := greeter.Greet(
//		ctx,
//		&greeterv0.HelloRequest{Name: "world"},
//	)
package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const engineTarget = "passthrough:///moby-engine"

// Engine supplies the engine connection dialer used by a Client.
//
// The dialer is kept lazy by the client and is invoked by gRPC only when the
// client is used.
type Engine interface {
	Dialer() func(context.Context) (net.Conn, error)
}

// Opt configures a Client.
type Opt func(*config) error

type config struct {
	points map[extensions.PointID]clientpoint.Registration
}

// WithGRPCPoint configures the generated ClientPoint registration for a point.
// Generated wire packages expose ClientPoint as the host-side gRPC wiring;
// Resolve receives the handwritten typed extensions.Point separately so its Go
// interface type remains available to the caller.
// Registration.Single is host cardinality metadata and is intentionally ignored
// by this client.
func WithGRPCPoint(registration clientpoint.Registration) Opt {
	return func(cfg *config) error {
		if cfg == nil {
			return errors.New("client: nil configuration")
		}
		if err := extensions.ValidatePointID(registration.Point); err != nil {
			return fmt.Errorf("client: invalid gRPC point registration: %w", err)
		}
		if registration.Provider == nil {
			return fmt.Errorf("client: point %q has nil provider", registration.Point)
		}
		if cfg.points == nil {
			cfg.points = make(map[extensions.PointID]clientpoint.Registration)
		}
		if _, exists := cfg.points[registration.Point]; exists {
			return fmt.Errorf("client: duplicate gRPC point registration for point %q", registration.Point)
		}
		cfg.points[registration.Point] = registration
		return nil
	}
}

type pointState struct {
	registration clientpoint.Registration
	provider     extensions.Provider
	ready        bool
}

// Client owns one lazy gRPC connection to the engine and the configured point
// adapters. The connection is private to the Client and is never borrowed from
// or closed by the Engine.
type Client struct {
	mu       sync.Mutex
	conn     *grpc.ClientConn
	points   map[extensions.PointID]*pointState
	closed   bool
	closeErr error
}

// New creates a Client without dialing the engine.
//
// The connection is separately owned by the returned Client and remains lazy;
// availability errors therefore occur when a resolved provider is invoked, not
// during construction. New performs no speculative transport connection or
// preflight.
func New(engine Engine, opts ...Opt) (*Client, error) {
	if isNil(engine) {
		return nil, errors.New("client: engine is nil")
	}

	cfg := config{points: make(map[extensions.PointID]clientpoint.Registration)}
	for _, opt := range opts {
		if opt == nil {
			return nil, errors.New("client: nil option")
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	dialer := engine.Dialer()
	if dialer == nil {
		return nil, errors.New("client: engine dialer is nil")
	}

	conn, err := grpc.NewClient(
		engineTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return dialer(ctx)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("client: create gRPC client: %w", err)
	}

	points := make(map[extensions.PointID]*pointState, len(cfg.points))
	for point, registration := range cfg.points {
		points[point] = &pointState{registration: registration}
	}
	return &Client{conn: conn, points: points}, nil
}

// Resolve returns the typed provider registered for point.
//
// The generated ClientPoint registration and handwritten extensions.Point are
// intentionally separate: ClientPoint supplies the gRPC adapter, while point
// supplies the compile-time provider interface type. The adapter is invoked at
// most once and only when this point is first resolved. Provider construction
// does not preflight the engine, so transport availability errors are returned
// by the provider's method invocation.
func Resolve[T any](client *Client, point extensions.Point[T]) (T, error) {
	var zero T
	if client == nil {
		return zero, errors.New("client: client is nil")
	}

	pointID := point.ID()
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.closed {
		return zero, errors.New("client: client is closed")
	}
	state, configured := client.points[pointID]
	if !configured {
		return zero, fmt.Errorf("client: point %q is not configured", pointID)
	}
	if !state.ready {
		state.ready = true
		state.provider = state.registration.Provider(client.conn)
	}

	provider := state.provider
	if provider.Point != pointID {
		return zero, fmt.Errorf("client: provider point %q does not match requested point %q", provider.Point, pointID)
	}
	impl, ok := provider.Impl.(T)
	if !ok {
		return zero, fmt.Errorf("client: provider for point %q has implementation type %T, want %v", pointID, provider.Impl, reflect.TypeFor[T]())
	}
	return impl, nil
}

// Close closes the Client's private gRPC connection.
//
// Close is nil-safe and idempotent. It waits for an in-progress Resolve before
// closing, and later Resolve calls fail without invoking another adapter.
func (client *Client) Close() error {
	if client == nil {
		return nil
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed {
		return client.closeErr
	}
	client.closed = true
	if client.conn == nil {
		return nil
	}
	client.closeErr = client.conn.Close()
	return client.closeErr
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
