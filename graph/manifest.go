package graph

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/trust"
	"github.com/docker/libtrust"
)

// loadManifest loads a manifest from a byte array and verifies its content,
// returning the local digest, the manifest itself, whether or not it was
// verified. If ref is a digest, rather than a tag, this will be treated as
// the local digest. An error will be returned if the signature verification
// fails, local digest verification fails and, if provided, the remote digest
// verification fails. The boolean return will only be false without error on
// the failure of signatures trust check.
func (s *TagStore) loadManifest(manifestBytes []byte, ref string, remoteDigest digest.Digest) (digest.Digest, *registry.ManifestData, bool, error) {
	payload, keys, err := unpackSignedManifest(manifestBytes)
	if err != nil {
		return "", nil, false, fmt.Errorf("error unpacking manifest: %v", err)
	}

	// TODO(stevvooe): It would be a lot better here to build up a stack of
	// verifiers, then push the bytes one time for signatures and digests, but
	// the manifests are typically small, so this optimization is not worth
	// hacking this code without further refactoring.

	var localDigest digest.Digest

	// Verify the local digest, if present in ref. ParseDigest will validate
	// that the ref is a digest and verify against that if present. Otherwize
	// (on error), we simply compute the localDigest and proceed.
	if dgst, err := digest.ParseDigest(ref); err == nil {
		// verify the manifest against local ref
		if err := verifyDigest(dgst, payload); err != nil {
			return "", nil, false, fmt.Errorf("verifying local digest: %v", err)
		}

		localDigest = dgst
	} else {
		// We don't have a local digest, since we are working from a tag.
		// Compute the digest of the payload and return that.
		logrus.Debugf("provided manifest reference %q is not a digest: %v", ref, err)
		localDigest, err = digest.FromBytes(payload)
		if err != nil {
			// near impossible
			logrus.Errorf("error calculating local digest during tag pull: %v", err)
			return "", nil, false, err
		}
	}

	// verify against the remote digest, if available
	if remoteDigest != "" {
		if err := verifyDigest(remoteDigest, payload); err != nil {
			return "", nil, false, fmt.Errorf("verifying remote digest: %v", err)
		}
	}

	var manifest registry.ManifestData
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return "", nil, false, fmt.Errorf("error unmarshalling manifest: %s", err)
	}

	// validate the contents of the manifest
	if err := validateManifest(&manifest); err != nil {
		return "", nil, false, err
	}

	var verified bool
	verified, err = s.verifyTrustedKeys(manifest.Name, keys)
	if err != nil {
		return "", nil, false, fmt.Errorf("error verifying trusted keys: %v", err)
	}

	return localDigest, &manifest, verified, nil
}

// unpackSignedManifest takes the raw, signed manifest bytes, unpacks the jws
// and returns the payload and public keys used to signed the manifest.
// Signatures are verified for authenticity but not against the trust store.
func unpackSignedManifest(p []byte) ([]byte, []libtrust.PublicKey, error) {
	sig, err := libtrust.ParsePrettySignature(p, "signatures")
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing payload: %s", err)
	}

	keys, err := sig.Verify()
	if err != nil {
		return nil, nil, fmt.Errorf("error verifying payload: %s", err)
	}

	payload, err := sig.Payload()
	if err != nil {
		return nil, nil, fmt.Errorf("error retrieving payload: %s", err)
	}

	return payload, keys, nil
}

// verifyTrustedKeys checks the keys provided against the trust store,
// ensuring that the provided keys are trusted for the namespace. The keys
// provided from this method must come from the signatures provided as part of
// the manifest JWS package, obtained from unpackSignedManifest or libtrust.
func (s *TagStore) verifyTrustedKeys(namespace string, keys []libtrust.PublicKey) (verified bool, err error) {
	if namespace[0] != '/' {
		namespace = "/" + namespace
	}

	for _, key := range keys {
		b, err := key.MarshalJSON()
		if err != nil {
			return false, fmt.Errorf("error marshalling public key: %s", err)
		}
		// Check key has read/write permission (0x03)
		v, err := s.trustService.CheckKey(namespace, b, 0x03)
		if err != nil {
			vErr, ok := err.(trust.NotVerifiedError)
			if !ok {
				return false, fmt.Errorf("error running key check: %s", err)
			}
			logrus.Debugf("Key check result: %v", vErr)
		}
		verified = v
	}

	if verified {
		logrus.Debug("Key check result: verified")
	}

	return
}

// verifyDigest checks the contents of p against the provided digest. Note
// that for manifests, this is the signed payload and not the raw bytes with
// signatures.
func verifyDigest(dgst digest.Digest, p []byte) error {
	if err := dgst.Validate(); err != nil {
		return fmt.Errorf("error validating  digest %q: %v", dgst, err)
	}

	verifier, err := digest.NewDigestVerifier(dgst)
	if err != nil {
		// There are not many ways this can go wrong: if it does, its
		// fatal. Likley, the cause would be poor validation of the
		// incoming reference.
		return fmt.Errorf("error creating verifier for digest %q: %v", dgst, err)
	}

	if _, err := verifier.Write(p); err != nil {
		return fmt.Errorf("error writing payload to digest verifier (verifier target %q): %v", dgst, err)
	}

	if !verifier.Verified() {
		return fmt.Errorf("verification against digest %q failed", dgst)
	}

	return nil
}

func validateManifest(manifest *registry.ManifestData) error {
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema version: %d", manifest.SchemaVersion)
	}

	if len(manifest.FSLayers) != len(manifest.History) {
		return fmt.Errorf("length of history not equal to number of layers")
	}

	if len(manifest.FSLayers) == 0 {
		return fmt.Errorf("no FSLayers in manifest")
	}

	return nil
}
