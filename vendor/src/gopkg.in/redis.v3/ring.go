package redis

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"gopkg.in/redis.v3/internal/consistenthash"
	"gopkg.in/redis.v3/internal/hashtag"
)

var (
	errRingShardsDown = errors.New("redis: all ring shards are down")
)

// RingOptions are used to configure a ring client and should be
// passed to NewRing.
type RingOptions struct {
	// A map of name => host:port addresses of ring shards.
	Addrs map[string]string

	// Following options are copied from Options struct.

	DB       int64
	Password string

	MaxRetries int

	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	PoolSize    int
	PoolTimeout time.Duration
	IdleTimeout time.Duration
}

func (opt *RingOptions) clientOptions() *Options {
	return &Options{
		DB:       opt.DB,
		Password: opt.Password,

		DialTimeout:  opt.DialTimeout,
		ReadTimeout:  opt.ReadTimeout,
		WriteTimeout: opt.WriteTimeout,

		PoolSize:    opt.PoolSize,
		PoolTimeout: opt.PoolTimeout,
		IdleTimeout: opt.IdleTimeout,
	}
}

type ringShard struct {
	Client *Client
	down   int
}

func (shard *ringShard) String() string {
	var state string
	if shard.IsUp() {
		state = "up"
	} else {
		state = "down"
	}
	return fmt.Sprintf("%s is %s", shard.Client, state)
}

func (shard *ringShard) IsDown() bool {
	const threshold = 5
	return shard.down >= threshold
}

func (shard *ringShard) IsUp() bool {
	return !shard.IsDown()
}

// Vote votes to set shard state and returns true if state was changed.
func (shard *ringShard) Vote(up bool) bool {
	if up {
		changed := shard.IsDown()
		shard.down = 0
		return changed
	}

	if shard.IsDown() {
		return false
	}

	shard.down++
	return shard.IsDown()
}

// Ring is a Redis client that uses constistent hashing to distribute
// keys across multiple Redis servers (shards). It's safe for
// concurrent use by multiple goroutines.
//
// Ring monitors the state of each shard and removes dead shards from
// the ring. When shard comes online it is added back to the ring. This
// gives you maximum availability and partition tolerance, but no
// consistency between different shards or even clients. Each client
// uses shards that are available to the client and does not do any
// coordination when shard state is changed.
//
// Ring should be used when you use multiple Redis servers for caching
// and can tolerate losing data when one of the servers dies.
// Otherwise you should use Redis Cluster.
type Ring struct {
	commandable

	opt       *RingOptions
	nreplicas int

	mx     sync.RWMutex
	hash   *consistenthash.Map
	shards map[string]*ringShard

	closed bool
}

func NewRing(opt *RingOptions) *Ring {
	const nreplicas = 100
	ring := &Ring{
		opt:       opt,
		nreplicas: nreplicas,

		hash:   consistenthash.New(nreplicas, nil),
		shards: make(map[string]*ringShard),
	}
	ring.commandable.process = ring.process
	for name, addr := range opt.Addrs {
		clopt := opt.clientOptions()
		clopt.Addr = addr
		ring.addClient(name, NewClient(clopt))
	}
	go ring.heartbeat()
	return ring
}

func (ring *Ring) addClient(name string, cl *Client) {
	ring.mx.Lock()
	ring.hash.Add(name)
	ring.shards[name] = &ringShard{Client: cl}
	ring.mx.Unlock()
}

func (ring *Ring) getClient(key string) (*Client, error) {
	ring.mx.RLock()

	if ring.closed {
		return nil, errClosed
	}

	name := ring.hash.Get(hashtag.Key(key))
	if name == "" {
		ring.mx.RUnlock()
		return nil, errRingShardsDown
	}

	cl := ring.shards[name].Client
	ring.mx.RUnlock()
	return cl, nil
}

func (ring *Ring) process(cmd Cmder) {
	cl, err := ring.getClient(cmd.clusterKey())
	if err != nil {
		cmd.setErr(err)
		return
	}
	cl.baseClient.process(cmd)
}

// rebalance removes dead shards from the ring.
func (ring *Ring) rebalance() {
	defer ring.mx.Unlock()
	ring.mx.Lock()

	ring.hash = consistenthash.New(ring.nreplicas, nil)
	for name, shard := range ring.shards {
		if shard.IsUp() {
			ring.hash.Add(name)
		}
	}
}

// heartbeat monitors state of each shard in the ring.
func (ring *Ring) heartbeat() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for _ = range ticker.C {
		var rebalance bool

		ring.mx.RLock()

		if ring.closed {
			ring.mx.RUnlock()
			break
		}

		for _, shard := range ring.shards {
			err := shard.Client.Ping().Err()
			if shard.Vote(err == nil || err == errPoolTimeout) {
				log.Printf("redis: ring shard state changed: %s", shard)
				rebalance = true
			}
		}

		ring.mx.RUnlock()

		if rebalance {
			ring.rebalance()
		}
	}
}

// Close closes the ring client, releasing any open resources.
//
// It is rare to Close a Ring, as the Ring is meant to be long-lived
// and shared between many goroutines.
func (ring *Ring) Close() (retErr error) {
	defer ring.mx.Unlock()
	ring.mx.Lock()

	if ring.closed {
		return nil
	}
	ring.closed = true

	for _, shard := range ring.shards {
		if err := shard.Client.Close(); err != nil {
			retErr = err
		}
	}
	ring.hash = nil
	ring.shards = nil

	return retErr
}

// RingPipeline creates a new pipeline which is able to execute commands
// against multiple shards. It's NOT safe for concurrent use by
// multiple goroutines.
type RingPipeline struct {
	commandable

	ring *Ring

	cmds   []Cmder
	closed bool
}

func (ring *Ring) Pipeline() *RingPipeline {
	pipe := &RingPipeline{
		ring: ring,
		cmds: make([]Cmder, 0, 10),
	}
	pipe.commandable.process = pipe.process
	return pipe
}

func (ring *Ring) Pipelined(fn func(*RingPipeline) error) ([]Cmder, error) {
	pipe := ring.Pipeline()
	if err := fn(pipe); err != nil {
		return nil, err
	}
	cmds, err := pipe.Exec()
	pipe.Close()
	return cmds, err
}

func (pipe *RingPipeline) process(cmd Cmder) {
	pipe.cmds = append(pipe.cmds, cmd)
}

// Discard resets the pipeline and discards queued commands.
func (pipe *RingPipeline) Discard() error {
	if pipe.closed {
		return errClosed
	}
	pipe.cmds = pipe.cmds[:0]
	return nil
}

// Exec always returns list of commands and error of the first failed
// command if any.
func (pipe *RingPipeline) Exec() (cmds []Cmder, retErr error) {
	if pipe.closed {
		return nil, errClosed
	}
	if len(pipe.cmds) == 0 {
		return pipe.cmds, nil
	}

	cmds = pipe.cmds
	pipe.cmds = make([]Cmder, 0, 10)

	cmdsMap := make(map[string][]Cmder)
	for _, cmd := range cmds {
		name := pipe.ring.hash.Get(hashtag.Key(cmd.clusterKey()))
		if name == "" {
			cmd.setErr(errRingShardsDown)
			if retErr == nil {
				retErr = errRingShardsDown
			}
			continue
		}
		cmdsMap[name] = append(cmdsMap[name], cmd)
	}

	for i := 0; i <= pipe.ring.opt.MaxRetries; i++ {
		failedCmdsMap := make(map[string][]Cmder)

		for name, cmds := range cmdsMap {
			client := pipe.ring.shards[name].Client
			cn, _, err := client.conn()
			if err != nil {
				setCmdsErr(cmds, err)
				if retErr == nil {
					retErr = err
				}
				continue
			}

			if i > 0 {
				resetCmds(cmds)
			}
			failedCmds, err := execCmds(cn, cmds)
			client.putConn(cn, err)
			if err != nil && retErr == nil {
				retErr = err
			}
			if len(failedCmds) > 0 {
				failedCmdsMap[name] = failedCmds
			}
		}

		if len(failedCmdsMap) == 0 {
			break
		}
		cmdsMap = failedCmdsMap
	}

	return cmds, retErr
}

// Close closes the pipeline, releasing any open resources.
func (pipe *RingPipeline) Close() error {
	pipe.Discard()
	pipe.closed = true
	return nil
}
