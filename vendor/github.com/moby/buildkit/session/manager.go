package session

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// Caller can invoke requests on the session
type Caller interface {
	Context() context.Context
	Supports(method string) bool
	Conn() *grpc.ClientConn
	Name() string
	SharedKey() string
}

type client struct {
	Session
	cc        *grpc.ClientConn
	supported map[string]struct{}
}

// Manager is a controller for accessing currently active sessions
type Manager struct {
	sessions        map[string]*client
	mu              sync.Mutex
	updateCondition *sync.Cond
}

// NewManager returns a new Manager
func NewManager() (*Manager, error) {
	sm := &Manager{
		sessions: make(map[string]*client),
	}
	sm.updateCondition = sync.NewCond(&sm.mu)
	return sm, nil
}

// HandleHTTPRequest handles an incoming HTTP request
func (sm *Manager) HandleHTTPRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return errors.New("handler does not support hijack")
	}

	uuid := r.Header.Get(headerSessionUUID)

	proto := r.Header.Get("Upgrade")

	sm.mu.Lock()
	if _, ok := sm.sessions[uuid]; ok {
		sm.mu.Unlock()
		return errors.Errorf("session %s already exists", uuid)
	}

	if proto == "" {
		sm.mu.Unlock()
		return errors.New("no upgrade proto in request")
	}

	if proto != "h2c" {
		sm.mu.Unlock()
		return errors.Errorf("protocol %s not supported", proto)
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		sm.mu.Unlock()
		return errors.Wrap(err, "failed to hijack connection")
	}

	resp := &http.Response{
		StatusCode: http.StatusSwitchingProtocols,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{},
	}
	resp.Header.Set("Connection", "Upgrade")
	resp.Header.Set("Upgrade", proto)

	// set raw mode
	conn.Write([]byte{})
	resp.Write(conn)

	return sm.handleConn(ctx, conn, r.Header)
}

// HandleConn handles an incoming raw connection
func (sm *Manager) HandleConn(ctx context.Context, conn net.Conn, opts map[string][]string) error {
	sm.mu.Lock()
	return sm.handleConn(ctx, conn, opts)
}

// caller needs to take lock, this function will release it
func (sm *Manager) handleConn(ctx context.Context, conn net.Conn, opts map[string][]string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	h := http.Header(opts)
	uuid := h.Get(headerSessionUUID)
	name := h.Get(headerSessionName)
	sharedKey := h.Get(headerSessionSharedKey)

	ctx, cc, err := grpcClientConn(ctx, conn)
	if err != nil {
		sm.mu.Unlock()
		return err
	}

	c := &client{
		Session: Session{
			uuid:      uuid,
			name:      name,
			sharedKey: sharedKey,
			ctx:       ctx,
			cancelCtx: cancel,
			done:      make(chan struct{}),
		},
		cc:        cc,
		supported: make(map[string]struct{}),
	}

	for _, m := range opts[headerSessionMethod] {
		c.supported[strings.ToLower(m)] = struct{}{}
	}
	sm.sessions[uuid] = c
	sm.updateCondition.Broadcast()
	sm.mu.Unlock()

	defer func() {
		sm.mu.Lock()
		delete(sm.sessions, uuid)
		sm.mu.Unlock()
	}()

	<-c.ctx.Done()
	conn.Close()
	close(c.done)

	return nil
}

// Get returns a session by UUID
func (sm *Manager) Get(ctx context.Context, uuid string) (Caller, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		select {
		case <-ctx.Done():
			sm.updateCondition.Broadcast()
		}
	}()

	var c *client

	sm.mu.Lock()
	for {
		select {
		case <-ctx.Done():
			sm.mu.Unlock()
			return nil, errors.Wrapf(ctx.Err(), "no active session for %s", uuid)
		default:
		}
		var ok bool
		c, ok = sm.sessions[uuid]
		if !ok || c.closed() {
			sm.updateCondition.Wait()
			continue
		}
		sm.mu.Unlock()
		break
	}

	return c, nil
}

func (c *client) Context() context.Context {
	return c.context()
}

func (c *client) Name() string {
	return c.name
}

func (c *client) SharedKey() string {
	return c.sharedKey
}

func (c *client) Supports(url string) bool {
	_, ok := c.supported[strings.ToLower(url)]
	return ok
}
func (c *client) Conn() *grpc.ClientConn {
	return c.cc
}
