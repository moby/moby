package daemon // import "github.com/docker/docker/daemon"

import "github.com/docker/libtrust"

// LoadOrCreateTrustKey attempts to load the libtrust key at the given path,
// otherwise generates a new one.
func loadOrCreateTrustKey(trustKeyPath string) (libtrust.PrivateKey, error) {
	return libtrust.LoadOrCreateTrustKey(trustKeyPath)
}
