package daemon

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
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
	ReadyCh() <-chan struct{}
}

type daemonDiscoveryReloader struct {
	backend discovery.Backend
	ticker  *time.Ticker
	term    chan bool
	readyCh chan struct{}
}

func (d *daemonDiscoveryReloader) Watch(stopCh <-chan struct{}) (<-chan discovery.Entries, <-chan error) {
	return d.backend.Watch(stopCh)
}

func (d *daemonDiscoveryReloader) ReadyCh() <-chan struct{} {
	return d.readyCh
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

		if h <= 0 {
			return time.Duration(0), time.Duration(0),
				fmt.Errorf("discovery.heartbeat must be positive")
		}

		heartbeat = time.Duration(h) * time.Second
		ttl = defaultDiscoveryTTLFactor * heartbeat
	}

	if tstr, ok := clusterOpts["discovery.ttl"]; ok {
		t, err := strconv.Atoi(tstr)
		if err != nil {
			return time.Duration(0), time.Duration(0), err
		}

		if t <= 0 {
			return time.Duration(0), time.Duration(0),
				fmt.Errorf("discovery.ttl must be positive")
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

// initDiscovery initializes the nodes discovery subsystem by connecting to the specified backend
// and starts a registration loop to advertise the current node under the specified address.
func initDiscovery(backendAddress, advertiseAddress string, clusterOpts map[string]string) (discoveryReloader, error) {
	heartbeat, backend, err := parseDiscoveryOptions(backendAddress, clusterOpts)
	if err != nil {
		return nil, err
	}

	reloader := &daemonDiscoveryReloader{
		backend: backend,
		ticker:  time.NewTicker(heartbeat),
		term:    make(chan bool),
		readyCh: make(chan struct{}),
	}
	// We call Register() on the discovery backend in a loop for the whole lifetime of the daemon,
	// but we never actually Watch() for nodes appearing and disappearing for the moment.
	go reloader.advertiseHeartbeat(advertiseAddress)
	return reloader, nil
}

// advertiseHeartbeat registers the current node against the discovery backend using the specified
// address. The function never returns, as registration against the backend comes with a TTL and
// requires regular heartbeats.
func (d *daemonDiscoveryReloader) advertiseHeartbeat(address string) {
	var ready bool
	if err := d.initHeartbeat(address); err == nil {
		ready = true
		close(d.readyCh)
	}

	for {
		select {
		case <-d.ticker.C:
			if err := d.backend.Register(address); err != nil {
				logrus.Warnf("Registering as %q in discovery failed: %v", address, err)
			} else {
				if !ready {
					close(d.readyCh)
					ready = true
				}
			}
		case <-d.term:
			return
		}
	}
}

// initHeartbeat is used to do the first heartbeat. It uses a tight loop until
// either the timeout period is reached or the heartbeat is successful and returns.
func (d *daemonDiscoveryReloader) initHeartbeat(address string) error {
	// Setup a short ticker until the first heartbeat has succeeded
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	// timeout makes sure that after a period of time we stop being so aggressive trying to reach the discovery service
	timeout := time.After(60 * time.Second)

	for {
		select {
		case <-timeout:
			return errors.New("timeout waiting for initial discovery")
		case <-d.term:
			return errors.New("terminated")
		case <-t.C:
			if err := d.backend.Register(address); err == nil {
				return nil
			}
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
	d.readyCh = make(chan struct{})

	go d.advertiseHeartbeat(advertiseAddress)
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
