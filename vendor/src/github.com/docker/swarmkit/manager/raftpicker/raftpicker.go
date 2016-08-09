package raftpicker

import (
	"sync"
	"time"

	"github.com/Sirupsen/logrus"

	"google.golang.org/grpc"
)

// Interface is interface to replace implementation with controlapi/hackpicker.
// TODO: it should be done cooler.
type Interface interface {
	Conn() (*grpc.ClientConn, error)
	Reset() error
}

// ConnSelector is struct for obtaining connection connected to cluster leader.
type ConnSelector struct {
	mu      sync.Mutex
	cluster RaftCluster
	opts    []grpc.DialOption

	cc   *grpc.ClientConn
	addr string

	stop chan struct{}
}

// NewConnSelector returns new ConnSelector with cluster and grpc.DialOpts which
// will be used for connection create.
func NewConnSelector(cluster RaftCluster, opts ...grpc.DialOption) *ConnSelector {
	cs := &ConnSelector{
		cluster: cluster,
		opts:    opts,
		stop:    make(chan struct{}),
	}
	go cs.updateLoop()
	return cs
}

// Conn returns *grpc.ClientConn which connected to cluster leader.
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
	cc, err := grpc.Dial(addr, c.opts...)
	if err != nil {
		return nil, err
	}
	c.cc = cc
	c.addr = addr
	return cc, nil
}

// Reset recreates underlying connection.
func (c *ConnSelector) Reset() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cc != nil {
		c.cc.Close()
		c.cc = nil
	}
	addr, err := c.cluster.LeaderAddr()
	if err != nil {
		logrus.WithError(err).Errorf("error obtaining leader address")
		return err
	}
	cc, err := grpc.Dial(addr, c.opts...)
	if err != nil {
		logrus.WithError(err).Errorf("error reestabilishing connection to leader")
		return err
	}
	c.cc = cc
	c.addr = addr
	return nil
}

// Stop cancels updating connection loop.
func (c *ConnSelector) Stop() {
	close(c.stop)
}

func (c *ConnSelector) updateConn() error {
	addr, err := c.cluster.LeaderAddr()
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.addr != addr {
		if c.cc != nil {
			c.cc.Close()
			c.cc = nil
		}
		conn, err := grpc.Dial(addr, c.opts...)
		if err != nil {
			return err
		}
		c.cc = conn
		c.addr = addr
	}
	return nil
}

func (c *ConnSelector) updateLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := c.updateConn(); err != nil {
				logrus.WithError(err).Errorf("error reestabilishing connection to leader")
			}
		case <-c.stop:
			return
		}
	}
}
