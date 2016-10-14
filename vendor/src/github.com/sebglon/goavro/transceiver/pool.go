package transceiver

import (
	"gopkg.in/fatih/pool.v2"
	"net"
	"fmt"
	"sync"
	"errors"
	"log"
	"time"
)


var (
	errPoolClosed = errors.New("Avro transceiver: Pool Closed")
)
type Pool struct {
	Config
	pool pool.Pool
	mu sync.RWMutex
	closed bool
}

const (
	defaultHost                   = "127.0.0.1"
	defaultNetwork                = "tcp"
	defaultSocketPath             = ""
	defaultPort                   = 63001
	defaultTimeout                = 3 * time.Second
	defaultBufferLimit            = 8 * 1024 * 1024
	defaultRetryWait              = 500
	defaultMaxRetry               = 13
	defaultInitialCap	      = 2
	defaultMaxCap		      = 5
	defaultReconnectWaitIncreRate = 1.5
)

func NewPool(config Config) (*Pool, error) {
	if config.Network == "" {
		config.Network = defaultNetwork
	}
	if config.Host == "" {
		config.Host = defaultHost
	}
	if config.Port == 0 {
		config.Port = defaultPort
	}
	if config.SocketPath == "" {
		config.SocketPath = defaultSocketPath
	}
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}
	if config.BufferLimit == 0 {
		config.BufferLimit = defaultBufferLimit
	}
	if config.RetryWait == 0 {
		config.RetryWait = defaultRetryWait
	}
	if config.MaxRetry == 0 {
		config.MaxRetry = defaultMaxRetry
	}
	if config.InitialCap == 0 {
		config.InitialCap = defaultInitialCap
	}
	if config.MaxCap == 0 {
		config.MaxCap = defaultMaxCap
	}
	p, err := pool.NewChannelPool(config.InitialCap,config.MaxCap, func() (net.Conn, error) {
		conn, err := NewConnection(config)
		if err != nil {
			return nil, fmt.Errorf("\nFail to init connec, %#v \n%v",config,err)
		}
		return conn, err
	})
	if err != nil {
		return nil, err
	}

	pool := &Pool{
		pool: p,
		Config: config,
	}
	log.Printf("%#v",pool.pool)
	return pool, nil

}

func (p *Pool) Conn() (*Connection, *pool.PoolConn, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, nil, errPoolClosed
	}


	nc, err := p.pool.Get()
	if err != nil {
		return nil, nil, err
	}

	log.Printf(" %T %#v",  nc,nc)

	pc, ok := nc.(*pool.PoolConn)
	if !ok {
		// This should never happen!
		return nil, nil, fmt.Errorf("Invalid connection in pool")
	}

	conn, ok := pc.Conn.(*Connection)
	if !ok {
		// This should never happen!
		return nil, nil, fmt.Errorf("Invalid connection in pool")
	}

	return conn, pc, nil
}

func (p *Pool) Call(conn *Connection, pc *pool.PoolConn, req []byte) (resp []byte, err error) {
	if err != nil {
		return
	}
	defer pc.Close()

	_,err= conn.Write(req)

	if err != nil {
		return nil, err
	}
	resp = make([]byte, 1024)
	_,err = conn.Read(resp)

	if err != nil {
		return nil, err
	}
	return
}
func (t *Pool) Close() {
	t.pool.Close()
}