package redis

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

//------------------------------------------------------------------------------

// FailoverOptions are used to configure a failover client and should
// be passed to NewFailoverClient.
type FailoverOptions struct {
	// The master name.
	MasterName string
	// A seed list of host:port addresses of sentinel nodes.
	SentinelAddrs []string

	// Following options are copied from Options struct.

	Password string
	DB       int64

	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	PoolSize    int
	PoolTimeout time.Duration
	IdleTimeout time.Duration

	MaxRetries int
}

func (opt *FailoverOptions) options() *Options {
	return &Options{
		Addr: "FailoverClient",

		DB:       opt.DB,
		Password: opt.Password,

		DialTimeout:  opt.DialTimeout,
		ReadTimeout:  opt.ReadTimeout,
		WriteTimeout: opt.WriteTimeout,

		PoolSize:    opt.PoolSize,
		PoolTimeout: opt.PoolTimeout,
		IdleTimeout: opt.IdleTimeout,

		MaxRetries: opt.MaxRetries,
	}
}

// NewFailoverClient returns a Redis client that uses Redis Sentinel
// for automatic failover. It's safe for concurrent use by multiple
// goroutines.
func NewFailoverClient(failoverOpt *FailoverOptions) *Client {
	opt := failoverOpt.options()
	failover := &sentinelFailover{
		masterName:    failoverOpt.MasterName,
		sentinelAddrs: failoverOpt.SentinelAddrs,

		opt: opt,
	}
	return newClient(opt, failover.Pool())
}

//------------------------------------------------------------------------------

type sentinelClient struct {
	commandable
	*baseClient
}

func newSentinel(opt *Options) *sentinelClient {
	base := &baseClient{
		opt:      opt,
		connPool: newConnPool(opt),
	}
	return &sentinelClient{
		baseClient:  base,
		commandable: commandable{process: base.process},
	}
}

func (c *sentinelClient) PubSub() *PubSub {
	return &PubSub{
		baseClient: &baseClient{
			opt:      c.opt,
			connPool: newStickyConnPool(c.connPool, false),
		},
	}
}

func (c *sentinelClient) GetMasterAddrByName(name string) *StringSliceCmd {
	cmd := NewStringSliceCmd("SENTINEL", "get-master-addr-by-name", name)
	c.Process(cmd)
	return cmd
}

func (c *sentinelClient) Sentinels(name string) *SliceCmd {
	cmd := NewSliceCmd("SENTINEL", "sentinels", name)
	c.Process(cmd)
	return cmd
}

type sentinelFailover struct {
	masterName    string
	sentinelAddrs []string

	opt *Options

	pool     pool
	poolOnce sync.Once

	lock      sync.RWMutex
	_sentinel *sentinelClient
}

func (d *sentinelFailover) dial() (net.Conn, error) {
	addr, err := d.MasterAddr()
	if err != nil {
		return nil, err
	}
	return net.DialTimeout("tcp", addr, d.opt.DialTimeout)
}

func (d *sentinelFailover) Pool() pool {
	d.poolOnce.Do(func() {
		d.opt.Dialer = d.dial
		d.pool = newConnPool(d.opt)
	})
	return d.pool
}

func (d *sentinelFailover) MasterAddr() (string, error) {
	defer d.lock.Unlock()
	d.lock.Lock()

	// Try last working sentinel.
	if d._sentinel != nil {
		addr, err := d._sentinel.GetMasterAddrByName(d.masterName).Result()
		if err != nil {
			log.Printf("redis-sentinel: GetMasterAddrByName %q failed: %s", d.masterName, err)
			d.resetSentinel()
		} else {
			addr := net.JoinHostPort(addr[0], addr[1])
			log.Printf("redis-sentinel: %q addr is %s", d.masterName, addr)
			return addr, nil
		}
	}

	for i, sentinelAddr := range d.sentinelAddrs {
		sentinel := newSentinel(&Options{
			Addr: sentinelAddr,

			DialTimeout:  d.opt.DialTimeout,
			ReadTimeout:  d.opt.ReadTimeout,
			WriteTimeout: d.opt.WriteTimeout,

			PoolSize:    d.opt.PoolSize,
			PoolTimeout: d.opt.PoolTimeout,
			IdleTimeout: d.opt.IdleTimeout,
		})
		masterAddr, err := sentinel.GetMasterAddrByName(d.masterName).Result()
		if err != nil {
			log.Printf("redis-sentinel: GetMasterAddrByName %q failed: %s", d.masterName, err)
			sentinel.Close()
			continue
		}

		// Push working sentinel to the top.
		d.sentinelAddrs[0], d.sentinelAddrs[i] = d.sentinelAddrs[i], d.sentinelAddrs[0]

		d.setSentinel(sentinel)
		addr := net.JoinHostPort(masterAddr[0], masterAddr[1])
		log.Printf("redis-sentinel: %q addr is %s", d.masterName, addr)
		return addr, nil
	}

	return "", errors.New("redis: all sentinels are unreachable")
}

func (d *sentinelFailover) setSentinel(sentinel *sentinelClient) {
	d.discoverSentinels(sentinel)
	d._sentinel = sentinel
	go d.listen()
}

func (d *sentinelFailover) discoverSentinels(sentinel *sentinelClient) {
	sentinels, err := sentinel.Sentinels(d.masterName).Result()
	if err != nil {
		log.Printf("redis-sentinel: Sentinels %q failed: %s", d.masterName, err)
		return
	}
	for _, sentinel := range sentinels {
		vals := sentinel.([]interface{})
		for i := 0; i < len(vals); i += 2 {
			key := vals[i].(string)
			if key == "name" {
				sentinelAddr := vals[i+1].(string)
				if !contains(d.sentinelAddrs, sentinelAddr) {
					log.Printf(
						"redis-sentinel: discovered new %q sentinel: %s",
						d.masterName, sentinelAddr,
					)
					d.sentinelAddrs = append(d.sentinelAddrs, sentinelAddr)
				}
			}
		}
	}
}

// closeOldConns closes connections to the old master after failover switch.
func (d *sentinelFailover) closeOldConns(newMaster string) {
	// Good connections that should be put back to the pool. They
	// can't be put immediately, because pool.First will return them
	// again on next iteration.
	cnsToPut := make([]*conn, 0)

	for {
		cn := d.pool.First()
		if cn == nil {
			break
		}
		if cn.RemoteAddr().String() != newMaster {
			err := fmt.Errorf(
				"redis-sentinel: closing connection to the old master %s",
				cn.RemoteAddr(),
			)
			log.Print(err)
			d.pool.Remove(cn, err)
		} else {
			cnsToPut = append(cnsToPut, cn)
		}
	}

	for _, cn := range cnsToPut {
		d.pool.Put(cn)
	}
}

func (d *sentinelFailover) listen() {
	var pubsub *PubSub
	for {
		if pubsub == nil {
			pubsub = d._sentinel.PubSub()
			if err := pubsub.Subscribe("+switch-master"); err != nil {
				log.Printf("redis-sentinel: Subscribe failed: %s", err)
				d.lock.Lock()
				d.resetSentinel()
				d.lock.Unlock()
				return
			}
		}

		msgIface, err := pubsub.Receive()
		if err != nil {
			log.Printf("redis-sentinel: Receive failed: %s", err)
			pubsub.Close()
			return
		}

		switch msg := msgIface.(type) {
		case *Message:
			switch msg.Channel {
			case "+switch-master":
				parts := strings.Split(msg.Payload, " ")
				if parts[0] != d.masterName {
					log.Printf("redis-sentinel: ignore new %s addr", parts[0])
					continue
				}
				addr := net.JoinHostPort(parts[3], parts[4])
				log.Printf(
					"redis-sentinel: new %q addr is %s",
					d.masterName, addr,
				)

				d.closeOldConns(addr)
			default:
				log.Printf("redis-sentinel: unsupported message: %s", msg)
			}
		case *Subscription:
			// Ignore.
		default:
			log.Printf("redis-sentinel: unsupported message: %s", msgIface)
		}
	}
}

func (d *sentinelFailover) resetSentinel() {
	d._sentinel.Close()
	d._sentinel = nil
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
