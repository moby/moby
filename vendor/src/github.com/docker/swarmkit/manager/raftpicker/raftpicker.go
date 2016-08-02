package raftpicker

import (
	"sync"
	"time"

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

	stop chan struct{}
	done chan struct{}
}

func newPicker(raft AddrSelector, addr string) *picker {
	return &picker{
		raft: raft,
		addr: addr,

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

// Init does initial processing for the Picker, e.g., initiate some connections.
func (p *picker) Init(cc *grpc.ClientConn) error {
	conn, err := grpc.NewConn(cc)
	if err != nil {
		return err
	}
	p.conn = conn
	return nil
}

// Pick blocks until either a transport.ClientTransport is ready for the upcoming RPC
// or some error happens.
func (p *picker) Pick(ctx context.Context) (transport.ClientTransport, error) {
	if err := p.updateConn(); err != nil {
		return nil, err
	}
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
	close(p.stop)
	<-p.done
	return p.conn.Close()
}

func (p *picker) updateConn() error {
	addr, err := p.raft.LeaderAddr()
	if err != nil {
		return err
	}
	p.mu.Lock()
	if p.addr != addr {
		p.addr = addr
		p.Reset()
	}
	p.mu.Unlock()
	return nil
}

func (p *picker) updateLoop() {
	defer close(p.done)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.updateConn()
		case <-p.stop:
			return
		}
	}
}

// ConnSelector is struct for obtaining connection with raftpicker.
type ConnSelector struct {
	mu      sync.Mutex
	cc      *grpc.ClientConn
	cluster RaftCluster
	opts    []grpc.DialOption
	picker  *picker
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
	c.picker = newPicker(c.cluster, addr)
	go c.picker.updateLoop()
	opts := append(c.opts, grpc.WithPicker(c.picker))
	cc, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}
	c.cc = cc
	return c.cc, nil
}

// Stop cancels tracking loop for raftpicker and closes it.
func (c *ConnSelector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.picker == nil {
		return
	}
	c.picker.Close()
}
