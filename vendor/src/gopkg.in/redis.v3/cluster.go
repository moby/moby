package redis

import (
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/redis.v3/internal/hashtag"
)

// ClusterClient is a Redis Cluster client representing a pool of zero
// or more underlying connections. It's safe for concurrent use by
// multiple goroutines.
type ClusterClient struct {
	commandable

	addrs   []string
	slots   [][]string
	slotsMx sync.RWMutex // Protects slots and addrs.

	clients   map[string]*Client
	closed    bool
	clientsMx sync.RWMutex // Protects clients and closed.

	opt *ClusterOptions

	// Reports where slots reloading is in progress.
	reloading uint32
}

// NewClusterClient returns a Redis Cluster client as described in
// http://redis.io/topics/cluster-spec.
func NewClusterClient(opt *ClusterOptions) *ClusterClient {
	client := &ClusterClient{
		addrs:   opt.Addrs,
		slots:   make([][]string, hashtag.SlotNumber),
		clients: make(map[string]*Client),
		opt:     opt,
	}
	client.commandable.process = client.process
	client.reloadSlots()
	go client.reaper()
	return client
}

// Watch creates new transaction and marks the keys to be watched
// for conditional execution of a transaction.
func (c *ClusterClient) Watch(keys ...string) (*Multi, error) {
	addr := c.slotMasterAddr(hashtag.Slot(keys[0]))
	client, err := c.getClient(addr)
	if err != nil {
		return nil, err
	}
	return client.Watch(keys...)
}

// Close closes the cluster client, releasing any open resources.
//
// It is rare to Close a ClusterClient, as the ClusterClient is meant
// to be long-lived and shared between many goroutines.
func (c *ClusterClient) Close() error {
	defer c.clientsMx.Unlock()
	c.clientsMx.Lock()

	if c.closed {
		return errClosed
	}
	c.closed = true
	c.resetClients()
	c.setSlots(nil)
	return nil
}

// getClient returns a Client for a given address.
func (c *ClusterClient) getClient(addr string) (*Client, error) {
	if addr == "" {
		return c.randomClient()
	}

	c.clientsMx.RLock()
	client, ok := c.clients[addr]
	if ok {
		c.clientsMx.RUnlock()
		return client, nil
	}
	c.clientsMx.RUnlock()

	c.clientsMx.Lock()
	if c.closed {
		c.clientsMx.Unlock()
		return nil, errClosed
	}

	client, ok = c.clients[addr]
	if !ok {
		opt := c.opt.clientOptions()
		opt.Addr = addr
		client = NewClient(opt)
		c.clients[addr] = client
	}
	c.clientsMx.Unlock()

	return client, nil
}

func (c *ClusterClient) slotAddrs(slot int) []string {
	c.slotsMx.RLock()
	addrs := c.slots[slot]
	c.slotsMx.RUnlock()
	return addrs
}

func (c *ClusterClient) slotMasterAddr(slot int) string {
	addrs := c.slotAddrs(slot)
	if len(addrs) > 0 {
		return addrs[0]
	}
	return ""
}

// randomClient returns a Client for the first live node.
func (c *ClusterClient) randomClient() (client *Client, err error) {
	for i := 0; i < 10; i++ {
		n := rand.Intn(len(c.addrs))
		client, err = c.getClient(c.addrs[n])
		if err != nil {
			continue
		}
		err = client.ClusterInfo().Err()
		if err == nil {
			return client, nil
		}
	}
	return nil, err
}

func (c *ClusterClient) process(cmd Cmder) {
	var ask bool

	slot := hashtag.Slot(cmd.clusterKey())

	addr := c.slotMasterAddr(slot)
	client, err := c.getClient(addr)
	if err != nil {
		cmd.setErr(err)
		return
	}

	for attempt := 0; attempt <= c.opt.getMaxRedirects(); attempt++ {
		if attempt > 0 {
			cmd.reset()
		}

		if ask {
			pipe := client.Pipeline()
			pipe.Process(NewCmd("ASKING"))
			pipe.Process(cmd)
			_, _ = pipe.Exec()
			pipe.Close()
			ask = false
		} else {
			client.Process(cmd)
		}

		// If there is no (real) error, we are done!
		err := cmd.Err()
		if err == nil || err == Nil || err == TxFailedErr {
			return
		}

		// On network errors try random node.
		if isNetworkError(err) {
			client, err = c.randomClient()
			if err != nil {
				return
			}
			continue
		}

		var moved bool
		var addr string
		moved, ask, addr = isMovedError(err)
		if moved || ask {
			if moved && c.slotMasterAddr(slot) != addr {
				c.lazyReloadSlots()
			}
			client, err = c.getClient(addr)
			if err != nil {
				return
			}
			continue
		}

		break
	}
}

// Closes all clients and returns last error if there are any.
func (c *ClusterClient) resetClients() (retErr error) {
	for addr, client := range c.clients {
		if err := client.Close(); err != nil && retErr == nil {
			retErr = err
		}
		delete(c.clients, addr)
	}
	return retErr
}

func (c *ClusterClient) setSlots(slots []ClusterSlotInfo) {
	c.slotsMx.Lock()

	seen := make(map[string]struct{})
	for _, addr := range c.addrs {
		seen[addr] = struct{}{}
	}

	for i := 0; i < hashtag.SlotNumber; i++ {
		c.slots[i] = c.slots[i][:0]
	}
	for _, info := range slots {
		for slot := info.Start; slot <= info.End; slot++ {
			c.slots[slot] = info.Addrs
		}

		for _, addr := range info.Addrs {
			if _, ok := seen[addr]; !ok {
				c.addrs = append(c.addrs, addr)
				seen[addr] = struct{}{}
			}
		}
	}

	c.slotsMx.Unlock()
}

func (c *ClusterClient) reloadSlots() {
	defer atomic.StoreUint32(&c.reloading, 0)

	client, err := c.randomClient()
	if err != nil {
		log.Printf("redis: randomClient failed: %s", err)
		return
	}

	slots, err := client.ClusterSlots().Result()
	if err != nil {
		log.Printf("redis: ClusterSlots failed: %s", err)
		return
	}
	c.setSlots(slots)
}

func (c *ClusterClient) lazyReloadSlots() {
	if !atomic.CompareAndSwapUint32(&c.reloading, 0, 1) {
		return
	}
	go c.reloadSlots()
}

// reaper closes idle connections to the cluster.
func (c *ClusterClient) reaper() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for _ = range ticker.C {
		c.clientsMx.RLock()

		if c.closed {
			c.clientsMx.RUnlock()
			break
		}

		for _, client := range c.clients {
			pool := client.connPool
			// pool.First removes idle connections from the pool and
			// returns first non-idle connection. So just put returned
			// connection back.
			if cn := pool.First(); cn != nil {
				pool.Put(cn)
			}
		}

		c.clientsMx.RUnlock()
	}
}

//------------------------------------------------------------------------------

// ClusterOptions are used to configure a cluster client and should be
// passed to NewClusterClient.
type ClusterOptions struct {
	// A seed list of host:port addresses of cluster nodes.
	Addrs []string

	// The maximum number of MOVED/ASK redirects to follow before
	// giving up.
	// Default is 16
	MaxRedirects int

	// Following options are copied from Options struct.

	Password string

	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	PoolSize    int
	PoolTimeout time.Duration
	IdleTimeout time.Duration
}

func (opt *ClusterOptions) getMaxRedirects() int {
	if opt.MaxRedirects == -1 {
		return 0
	}
	if opt.MaxRedirects == 0 {
		return 16
	}
	return opt.MaxRedirects
}

func (opt *ClusterOptions) clientOptions() *Options {
	return &Options{
		Password: opt.Password,

		DialTimeout:  opt.DialTimeout,
		ReadTimeout:  opt.ReadTimeout,
		WriteTimeout: opt.WriteTimeout,

		PoolSize:    opt.PoolSize,
		PoolTimeout: opt.PoolTimeout,
		IdleTimeout: opt.IdleTimeout,
	}
}
