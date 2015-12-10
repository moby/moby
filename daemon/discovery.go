package daemon

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/discovery"

	// Register the libkv backends for discovery.
	_ "github.com/docker/docker/pkg/discovery/kv"
)

const (
	// defaultDiscoveryHeartbeat is the default value for discovery heartbeat interval.
	defaultDiscoveryHeartbeat = 20 * time.Second
	// defaultDiscoveryTTLFactor is the default TTL factor for discovery
	defaultDiscoveryTTLFactor = 3
)

var errDiscoveryDisabled = errors.New("discovery is disabled")

type discoveryReloader interface {
	discovery.Watcher
	Stop()
	Reload(backend, address string, clusterOpts map[string]string) error
}

type daemonDiscoveryReloader struct {
	backend discovery.Backend
	ticker  *time.Ticker
	term    chan bool
}

func (d *daemonDiscoveryReloader) Watch(stopCh <-chan struct{}) (<-chan discovery.Entries, <-chan error) {
	return d.backend.Watch(stopCh)
}

func discoveryOpts(clusterOpts map[string]string) (time.Duration, time.Duration, error) {
	var (
		heartbeat = defaultDiscoveryHeartbeat
		ttl       = defaultDiscoveryTTLFactor * defaultDiscoveryHeartbeat
	)

	if hb, ok := clusterOpts["discovery.heartbeat"]; ok {
		h, err := strconv.Atoi(hb)
		if err != nil {
			return time.Duration(0), time.Duration(0), err
		}
		heartbeat = time.Duration(h) * time.Second
		ttl = defaultDiscoveryTTLFactor * heartbeat
	}

	if tstr, ok := clusterOpts["discovery.ttl"]; ok {
		t, err := strconv.Atoi(tstr)
		if err != nil {
			return time.Duration(0), time.Duration(0), err
		}
		ttl = time.Duration(t) * time.Second

		if _, ok := clusterOpts["discovery.heartbeat"]; !ok {
			h := int(t / defaultDiscoveryTTLFactor)
			heartbeat = time.Duration(h) * time.Second
		}

		if ttl <= heartbeat {
			return time.Duration(0), time.Duration(0),
				fmt.Errorf("discovery.ttl timer must be greater than discovery.heartbeat")
		}
	}

	return heartbeat, ttl, nil
}

// initDiscovery initialized the nodes discovery subsystem by connecting to the specified backend
// and start a registration loop to advertise the current node under the specified address.
func initDiscovery(backendAddress, advertiseAddress string, clusterOpts map[string]string) (discoveryReloader, error) {
	heartbeat, backend, err := parseDiscoveryOptions(backendAddress, clusterOpts)
	if err != nil {
		return nil, err
	}

	reloader := &daemonDiscoveryReloader{
		backend: backend,
		ticker:  time.NewTicker(heartbeat),
		term:    make(chan bool),
	}
	// We call Register() on the discovery backend in a loop for the whole lifetime of the daemon,
	// but we never actually Watch() for nodes appearing and disappearing for the moment.
	reloader.advertise(advertiseAddress)
	return reloader, nil
}

func (d *daemonDiscoveryReloader) advertise(address string) {
	d.registerAddr(address)
	go d.advertiseHeartbeat(address)
}

func (d *daemonDiscoveryReloader) registerAddr(addr string) {
	if err := d.backend.Register(addr); err != nil {
		log.Warnf("Registering as %q in discovery failed: %v", addr, err)
	}
}

// advertiseHeartbeat registers the current node against the discovery backend using the specified
// address. The function never returns, as registration against the backend comes with a TTL and
// requires regular heartbeats.
func (d *daemonDiscoveryReloader) advertiseHeartbeat(address string) {
	for {
		select {
		case <-d.ticker.C:
			d.registerAddr(address)
		case <-d.term:
			return
		}
	}
}

// Reload makes the watcher to stop advertising and reconfigures it to advertise in a new address.
func (d *daemonDiscoveryReloader) Reload(backendAddress, advertiseAddress string, clusterOpts map[string]string) error {
	d.Stop()

	heartbeat, backend, err := parseDiscoveryOptions(backendAddress, clusterOpts)
	if err != nil {
		return err
	}

	d.backend = backend
	d.ticker = time.NewTicker(heartbeat)

	d.advertise(advertiseAddress)
	return nil
}

// Stop terminates the discovery advertising.
func (d *daemonDiscoveryReloader) Stop() {
	d.ticker.Stop()
	d.term <- true
}

func parseDiscoveryOptions(backendAddress string, clusterOpts map[string]string) (time.Duration, discovery.Backend, error) {
	heartbeat, ttl, err := discoveryOpts(clusterOpts)
	if err != nil {
		return 0, nil, err
	}

	backend, err := discovery.New(backendAddress, heartbeat, ttl, clusterOpts)
	if err != nil {
		return 0, nil, err
	}
	return heartbeat, backend, nil
}

// modifiedDiscoverySettings returns whether the discovery configuration has been modified or not.
func modifiedDiscoverySettings(config *Config, backendType, advertise string, clusterOpts map[string]string) bool {
	if config.ClusterStore != backendType || config.ClusterAdvertise != advertise {
		return true
	}

	if (config.ClusterOpts == nil && clusterOpts == nil) ||
		(config.ClusterOpts == nil && len(clusterOpts) == 0) ||
		(len(config.ClusterOpts) == 0 && clusterOpts == nil) {
		return false
	}

	return !reflect.DeepEqual(config.ClusterOpts, clusterOpts)
}
