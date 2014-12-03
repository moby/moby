package trust

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/libtrust"
)

func (t *TrustStore) Install(eng *engine.Engine) error {
	for name, handler := range map[string]engine.Handler{
		"trust_key_check":   t.CmdCheckKey,
		"trust_update_base": t.CmdUpdateBase,
	} {
		if err := eng.Register(name, handler); err != nil {
			return fmt.Errorf("Could not register %q: %v", name, err)
		}
	}
	return nil
}

func (t *TrustStore) CmdCheckKey(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: %s NAMESPACE", job.Name)
	}
	var (
		namespace = job.Args[0]
		keyBytes  = job.Getenv("PublicKey")
	)

	if keyBytes == "" {
		return job.Errorf("Missing PublicKey")
	}
	pk, err := libtrust.UnmarshalPublicKeyJWK([]byte(keyBytes))
	if err != nil {
		return job.Errorf("Error unmarshalling public key: %s", err)
	}

	permission := uint16(job.GetenvInt("Permission"))
	if permission == 0 {
		permission = 0x03
	}

	t.RLock()
	defer t.RUnlock()
	if t.graph == nil {
		job.Stdout.Write([]byte("no graph"))
		return engine.StatusOK
	}

	// Check if any expired grants
	verified, err := t.graph.Verify(pk, namespace, permission)
	if err != nil {
		return job.Errorf("Error verifying key to namespace: %s", namespace)
	}
	if !verified {
		log.Debugf("Verification failed for %s using key %s", namespace, pk.KeyID())
		job.Stdout.Write([]byte("not verified"))
	} else if t.expiration.Before(time.Now()) {
		job.Stdout.Write([]byte("expired"))
	} else {
		job.Stdout.Write([]byte("verified"))
	}

	return engine.StatusOK
}

func (t *TrustStore) CmdUpdateBase(job *engine.Job) engine.Status {
	t.fetch()

	return engine.StatusOK
}
