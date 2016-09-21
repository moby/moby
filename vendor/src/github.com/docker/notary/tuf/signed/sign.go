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

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary/trustmanager"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/utils"
)

// Sign takes a data.Signed and a cryptoservice containing private keys,
// calculates and adds at least minSignature signatures using signingKeys the
// data.Signed.  It will also clean up any signatures that are not in produced
// by either a signingKey or an otherWhitelistedKey.
// Note that in most cases, otherWhitelistedKeys should probably be null. They
// are for keys you don't want to sign with, but you also don't want to remove
// existing signatures by those keys.  For instance, if you want to call Sign
// multiple times with different sets of signing keys without undoing removing
// signatures produced by the previous call to Sign.
func Sign(service CryptoService, s *data.Signed, signingKeys []data.PublicKey,
	minSignatures int, otherWhitelistedKeys []data.PublicKey) error {

	logrus.Debugf("sign called with %d/%d required keys", minSignatures, len(signingKeys))
	signatures := make([]data.Signature, 0, len(s.Signatures)+1)
	signingKeyIDs := make(map[string]struct{})
	tufIDs := make(map[string]data.PublicKey)

	privKeys := make(map[string]data.PrivateKey)

	// Get all the private key objects related to the public keys
	missingKeyIDs := []string{}
	for _, key := range signingKeys {
		canonicalID, err := utils.CanonicalKeyID(key)
		tufIDs[key.ID()] = key
		if err != nil {
			return err
		}
		k, _, err := service.GetPrivateKey(canonicalID)
		if err != nil {
			if _, ok := err.(trustmanager.ErrKeyNotFound); ok {
				missingKeyIDs = append(missingKeyIDs, canonicalID)
				continue
			}
			return err
		}
		privKeys[key.ID()] = k
	}

	// include the list of otherWhitelistedKeys
	for _, key := range otherWhitelistedKeys {
		if _, ok := tufIDs[key.ID()]; !ok {
			tufIDs[key.ID()] = key
		}
	}

	// Check to ensure we have enough signing keys
	if len(privKeys) < minSignatures {
		return ErrInsufficientSignatures{FoundKeys: len(privKeys),
			NeededKeys: minSignatures, MissingKeyIDs: missingKeyIDs}
	}

	emptyStruct := struct{}{}
	// Do signing and generate list of signatures
	for keyID, pk := range privKeys {
		sig, err := pk.Sign(rand.Reader, *s.Signed, nil)
		if err != nil {
			logrus.Debugf("Failed to sign with key: %s. Reason: %v", keyID, err)
			return err
		}
		signingKeyIDs[keyID] = emptyStruct
		signatures = append(signatures, data.Signature{
			KeyID:     keyID,
			Method:    pk.SignatureAlgorithm(),
			Signature: sig[:],
		})
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
		if err := VerifySignature(*s.Signed, &sig, k); err != nil {
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
