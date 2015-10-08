package discovery

import (
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

var (
	// Backends is a global map of discovery backends indexed by their
	// associated scheme.
	backends map[string]Backend
)

func init() {
	backends = make(map[string]Backend)
}

// Register makes a discovery backend available by the provided scheme.
// If Register is called twice with the same scheme an error is returned.
func Register(scheme string, d Backend) error {
	if _, exists := backends[scheme]; exists {
		return fmt.Errorf("scheme already registered %s", scheme)
	}
	log.WithField("name", scheme).Debug("Registering discovery service")
	backends[scheme] = d
	return nil
}

func parse(rawurl string) (string, string) {
	parts := strings.SplitN(rawurl, "://", 2)

	// nodes:port,node2:port => nodes://node1:port,node2:port
	if len(parts) == 1 {
		return "nodes", parts[0]
	}
	return parts[0], parts[1]
}

// New returns a new Discovery given a URL, heartbeat and ttl settings.
// Returns an error if the URL scheme is not supported.
func New(rawurl string, heartbeat time.Duration, ttl time.Duration, clusterOpts map[string]string) (Backend, error) {
	scheme, uri := parse(rawurl)
	if backend, exists := backends[scheme]; exists {
		log.WithFields(log.Fields{"name": scheme, "uri": uri}).Debug("Initializing discovery service")
		err := backend.Initialize(uri, heartbeat, ttl, clusterOpts)
		return backend, err
	}

	return nil, ErrNotSupported
}
