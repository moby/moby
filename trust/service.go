package trust

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libtrust"
)

// NotVerifiedError reports a error when doing the key check.
// For example if the graph is not verified or the key has expired.
type NotVerifiedError string

func (e NotVerifiedError) Error() string {
	return string(e)
}

// CheckKey verifies that the given public key is allowed to perform
// the given action on the given node according to the trust graph.
func (t *Store) CheckKey(ns string, key []byte, perm uint16) (bool, error) {
	if len(key) == 0 {
		return false, fmt.Errorf("Missing PublicKey")
	}
	pk, err := libtrust.UnmarshalPublicKeyJWK(key)
	if err != nil {
		return false, fmt.Errorf("Error unmarshalling public key: %v", err)
	}

	if perm == 0 {
		perm = 0x03
	}

	t.RLock()
	defer t.RUnlock()
	if t.graph == nil {
		return false, NotVerifiedError("no graph")
	}

	// Check if any expired grants
	verified, err := t.graph.Verify(pk, ns, perm)
	if err != nil {
		return false, fmt.Errorf("Error verifying key to namespace: %s", ns)
	}
	if !verified {
		logrus.Debugf("Verification failed for %s using key %s", ns, pk.KeyID())
		return false, NotVerifiedError("not verified")
	}
	if t.expiration.Before(time.Now()) {
		return false, NotVerifiedError("expired")
	}
	return true, nil
}

// UpdateBase retrieves updated base graphs. This function cannot error, it
// should only log errors.
func (t *Store) UpdateBase() {
	t.fetch()
}
