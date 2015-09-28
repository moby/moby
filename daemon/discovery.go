package daemon

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/discovery"

	// Register the libkv backends for discovery.
	_ "github.com/docker/docker/pkg/discovery/kv"
)

const (
	// defaultDiscoveryHeartbeat is the default value for discovery heartbeat interval.
	defaultDiscoveryHeartbeat = 20 * time.Second

	// defaultDiscoveryTTL is the default TTL interface for discovery.
	defaultDiscoveryTTL = 60 * time.Second
)

// initDiscovery initialized the nodes discovery subsystem by connecting to the specified backend
// and start a registration loop to advertise the current node under the specified address.
func initDiscovery(backend, address string, clusterOpts map[string]string) (discovery.Backend, error) {
	var (
		discoveryBackend discovery.Backend
		err              error
	)
	if discoveryBackend, err = discovery.New(backend, defaultDiscoveryHeartbeat, defaultDiscoveryTTL, clusterOpts); err != nil {
		return nil, err
	}

	// We call Register() on the discovery backend in a loop for the whole lifetime of the daemon,
	// but we never actually Watch() for nodes appearing and disappearing for the moment.
	go registrationLoop(discoveryBackend, address)
	return discoveryBackend, nil
}

// registrationLoop registers the current node against the discovery backend using the specified
// address. The function never returns, as registration against the backend comes with a TTL and
// requires regular heartbeats.
func registrationLoop(discoveryBackend discovery.Backend, address string) {
	for {
		if err := discoveryBackend.Register(address); err != nil {
			log.Errorf("Registering as %q in discovery failed: %v", address, err)
		}
		time.Sleep(defaultDiscoveryHeartbeat)
	}
}
