package daemon

import (
	"fmt"
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
func initDiscovery(backend, address string, clusterOpts map[string]string) (discovery.Backend, error) {

	heartbeat, ttl, err := discoveryOpts(clusterOpts)
	if err != nil {
		return nil, err
	}

	discoveryBackend, err := discovery.New(backend, heartbeat, ttl, clusterOpts)
	if err != nil {
		return nil, err
	}

	// We call Register() on the discovery backend in a loop for the whole lifetime of the daemon,
	// but we never actually Watch() for nodes appearing and disappearing for the moment.
	go registrationLoop(discoveryBackend, address, heartbeat)
	return discoveryBackend, nil
}

func registerAddr(backend discovery.Backend, addr string) {
	if err := backend.Register(addr); err != nil {
		log.Warnf("Registering as %q in discovery failed: %v", addr, err)
	}
}

// registrationLoop registers the current node against the discovery backend using the specified
// address. The function never returns, as registration against the backend comes with a TTL and
// requires regular heartbeats.
func registrationLoop(discoveryBackend discovery.Backend, address string, heartbeat time.Duration) {
	registerAddr(discoveryBackend, address)
	for range time.Tick(heartbeat) {
		registerAddr(discoveryBackend, address)
	}
}
