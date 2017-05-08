// Package hackpicker is temporary solution to provide more seamless experience
// for controlapi. It has drawback of slow reaction to leader change, but it
// tracks leader automatically without erroring out to client.
package hackpicker

import (
	"sync"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/transport"
)

// picker always picks address of cluster leader.
type picker struct {
	mu   sync.Mutex
	addr string
	raft AddrSelector
	conn *grpc.Conn
	cc   *grpc.ClientConn
}

// Init does initial processing for the Picker, e.g., initiate some connections.
func (p *picker) Init(cc *grpc.ClientConn) error {
	p.cc = cc
	return nil
}

func (p *picker) initConn() error {
	if p.conn == nil {
		conn, err := grpc.NewConn(p.cc)
		if err != nil {
			return err
		}
		p.conn = conn
	}
	return nil
}

// Pick blocks until either a transport.ClientTransport is ready for the upcoming RPC
// or some error happens.
func (p *picker) Pick(ctx context.Context) (transport.ClientTransport, error) {
	p.mu.Lock()
	if err := p.initConn(); err != nil {
		p.mu.Unlock()
		return nil, err
	}
	p.mu.Unlock()

	addr, err := p.raft.LeaderAddr()
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	if p.addr != addr {
		p.addr = addr
		p.conn.NotifyReset()
	}
	p.mu.Unlock()
	return p.conn.Wait(ctx)
}

// PickAddr picks a peer address for connecting. This will be called repeated for
// connecting/reconnecting.
func (p *picker) PickAddr() (string, error) {
	addr, err := p.raft.LeaderAddr()
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	p.addr = addr
	p.mu.Unlock()
	return addr, nil
}

// State returns the connectivity state of the underlying connections.
func (p *picker) State() (grpc.ConnectivityState, error) {
	return p.conn.State(), nil
}

// WaitForStateChange blocks until the state changes to something other than
// the sourceState. It returns the new state or error.
func (p *picker) WaitForStateChange(ctx context.Context, sourceState grpc.ConnectivityState) (grpc.ConnectivityState, error) {
	return p.conn.WaitForStateChange(ctx, sourceState)
}

// Reset the current connection and force a reconnect to another address.
func (p *picker) Reset() error {
	p.conn.NotifyReset()
	return nil
}

// Close closes all the Conn's owned by this Picker.
func (p *picker) Close() error {
	return p.conn.Close()
}

// ConnSelector is struct for obtaining connection with raftpicker.
type ConnSelector struct {
	mu      sync.Mutex
	cc      *grpc.ClientConn
	cluster RaftCluster
	opts    []grpc.DialOption
}

// NewConnSelector returns new ConnSelector with cluster and grpc.DialOpts which
// will be used for Dial on first call of Conn.
func NewConnSelector(cluster RaftCluster, opts ...grpc.DialOption) *ConnSelector {
	return &ConnSelector{
		cluster: cluster,
		opts:    opts,
	}
}

// Conn returns *grpc.ClientConn with picker which picks raft cluster leader.
// Internal connection estabilished lazily on this call.
// It can return error if cluster wasn't ready at the moment of initial call.
func (c *ConnSelector) Conn() (*grpc.ClientConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cc != nil {
		return c.cc, nil
	}
	addr, err := c.cluster.LeaderAddr()
	if err != nil {
		return nil, err
	}
	picker := &picker{raft: c.cluster, addr: addr}
	opts := append(c.opts, grpc.WithPicker(picker))
	cc, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}
	c.cc = cc
	return c.cc, nil
}

// Reset does nothing for hackpicker.
func (c *ConnSelector) Reset() error {
	return nil
}
