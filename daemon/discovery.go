package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/discovery"

	// Register the libkv backends for discovery.
	_ "github.com/docker/docker/pkg/discovery/kv"
)

const (
	// defaultDiscoveryHeartbeat is the default value for discovery heartbeat interval.
	defaultDiscoveryHeartbeat = 20 * time.Second

	// defaultDiscoveryTTL is the default TTL interface for discovery.
	defaultDiscoveryTTL = 60 * time.Second

	// Persisted discovery config
	discoveryConfigFile = "discovery/config.json"
)

var (
	ErrDaemonAssociated = errors.New("The Docker daemon is already associated to a discovery backend")
)

func (daemon *Daemon) Join(config types.DiscoveryConfig) error {
	if daemon.associated {
		return ErrDaemonAssociated
	}

	// Discovery is only enabled when the daemon is launched with a given discovery address to be
	// advertised to the backend. When initialized, the daemon is registered and we can store the
	// discovery backend as its read-only DiscoveryWatcher version.
	if config.Address != "" {
		var err error
		if daemon.discoveryWatcher, err = initDiscovery(config.Backend, config.Address); err != nil {
			return fmt.Errorf("discovery initialization failed (%v)", err)
		}
		daemon.associated = true
	}

	return nil
}

func (daemon *Daemon) SaveDiscoveryConfig(config types.DiscoveryConfig) error {
	configFile := path.Join(daemon.configStore.Root, discoveryConfigFile)

	if err := os.MkdirAll(path.Dir(configFile), 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(&config); err != nil {
		return err
	}

	daemon.configStore.Discovery = config
	return nil
}

func initDiscoveryConfig(config *Config) error {
	// Initialize discovery. When a discovery backend is not specified, we rely on the network
	// key-value store by default.
	if config.Discovery.Backend == "" {
		config.Discovery.Backend = config.Discovery.NetworkKVStore
	}

	if config.Discovery.IsEmpty() {
		f, err := os.Open(path.Join(config.Root, discoveryConfigFile))
		if os.IsNotExist(err) {
			return nil
		}
		defer f.Close()

		if err := json.NewDecoder(f).Decode(&config.Discovery); err != nil {
			return err
		}
	}

	return nil
}

// initDiscovery initialized the nodes discovery subsystem by connecting to the specified backend
// and start a registration loop to advertise the current node under the specified address.
func initDiscovery(backend, address string) (discovery.Backend, error) {
	var (
		discoveryBackend discovery.Backend
		err              error
	)
	if discoveryBackend, err = discovery.New(backend, defaultDiscoveryHeartbeat, defaultDiscoveryTTL); err != nil {
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
