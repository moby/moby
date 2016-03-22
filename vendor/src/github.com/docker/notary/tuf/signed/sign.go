package signed

// The Sign function is a choke point for all code paths that do signing.
// We use this fact to do key ID translation. There are 2 types of key ID:
//   - Scoped: the key ID based purely on the data that appears in the TUF
//             files. This may be wrapped by a certificate that scopes the
//             key to be used in a specific context.
//   - Canonical: the key ID based purely on the public key bytes. This is
//             used by keystores to easily identify keys that may be reused
//             in many scoped locations.
// Currently these types only differ in the context of Root Keys in Notary
// for which the root key is wrapped using an x509 certificate.

import (
	"crypto/rand"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/utils"
)

// Sign takes a data.Signed and a key, calculated and adds the signature
// to the data.Signed
// N.B. All public keys for a role should be passed so that this function
//      can correctly clean up signatures that are no longer valid.
func Sign(service CryptoService, s *data.Signed, keys ...data.PublicKey) error {
	logrus.Debugf("sign called with %d keys", len(keys))
	signatures := make([]data.Signature, 0, len(s.Signatures)+1)
	signingKeyIDs := make(map[string]struct{})
	tufIDs := make(map[string]data.PublicKey)
	ids := make([]string, 0, len(keys))

	privKeys := make(map[string]data.PrivateKey)

	// Get all the private key objects related to the public keys
	for _, key := range keys {
		canonicalID, err := utils.CanonicalKeyID(key)
		ids = append(ids, canonicalID)
		tufIDs[key.ID()] = key
		if err != nil {
			continue
		}
		k, _, err := service.GetPrivateKey(canonicalID)
		if err != nil {
			continue
		}
		privKeys[key.ID()] = k
	}

	// Check to ensure we have at least one signing key
	if len(privKeys) == 0 {
		return ErrNoKeys{KeyIDs: ids}
	}

	// Do signing and generate list of signatures
	for keyID, pk := range privKeys {
		sig, err := pk.Sign(rand.Reader, *s.Signed, nil)
		if err != nil {
			logrus.Debugf("Failed to sign with key: %s. Reason: %v", keyID, err)
			continue
		}
		signingKeyIDs[keyID] = struct{}{}
		signatures = append(signatures, data.Signature{
			KeyID:     keyID,
			Method:    pk.SignatureAlgorithm(),
			Signature: sig[:],
		})
	}

	// Check we produced at least on signature
	if len(signatures) < 1 {
		return ErrInsufficientSignatures{
			Name: fmt.Sprintf(
				"cryptoservice failed to produce any signatures for keys with IDs: %v",
				ids),
		}
	}

	for _, sig := range s.Signatures {
		if _, ok := signingKeyIDs[sig.KeyID]; ok {
			// key is in the set of key IDs for which a signature has been created
			continue
		}
		var (
			k  data.PublicKey
			ok bool
		)
		if k, ok = tufIDs[sig.KeyID]; !ok {
			// key is no longer a valid signing key
			continue
		}
		if err := VerifySignature(*s.Signed, sig, k); err != nil {
			// signature is no longer valid
			continue
		}
		// keep any signatures that still represent valid keys and are
		// themselves valid
		signatures = append(signatures, sig)
	}
	s.Signatures = signatures
	return nil
}
