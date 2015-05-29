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
	sig, err := libtrust.ParsePrettySignature(manifestBytes, "signatures")
	if err != nil {
		return "", nil, false, fmt.Errorf("error parsing payload: %s", err)
	}

	keys, err := sig.Verify()
	if err != nil {
		return "", nil, false, fmt.Errorf("error verifying payload: %s", err)
	}

	payload, err := sig.Payload()
	if err != nil {
		return "", nil, false, fmt.Errorf("error retrieving payload: %s", err)
	}

	// TODO(stevvooe): It would be a lot better here to build up a stack of
	// verifiers, then push the bytes one time for signatures and digests, but
	// the manifests are typically small, so this optimization is not worth
	// hacking this code without further refactoring.

	var localDigest digest.Digest

	// verify the local digest, if present in tag
	if dgst, err := digest.ParseDigest(ref); err != nil {
		logrus.Debugf("provided manifest reference %q is not a digest: %v", ref, err)

		// we don't have a local digest, since we are working from a tag.
		// Digest the payload and return that.
		localDigest, err = digest.FromBytes(payload)
		if err != nil {
			// near impossible
			logrus.Errorf("error calculating local digest during tag pull: %v", err)
			return "", nil, false, err
		}
	} else {
		// verify the manifest against local ref
		verifier, err := digest.NewDigestVerifier(dgst)
		if err != nil {
			// There are not many ways this can go wrong: if it does, its
			// fatal. Likley, the cause would be poor validation of the
			// incoming reference.
			return "", nil, false, fmt.Errorf("error creating verifier for local digest reference %q: %v", dgst, err)
		}

		if _, err := verifier.Write(payload); err != nil {
			return "", nil, false, fmt.Errorf("error writing payload to local digest reference verifier: %v", err)
		}

		if !verifier.Verified() {
			return "", nil, false, fmt.Errorf("verification against local digest reference %q failed", dgst)
		}

		localDigest = dgst
	}

	// verify against the remote digest, if available
	if remoteDigest != "" {
		if err := remoteDigest.Validate(); err != nil {
			return "", nil, false, fmt.Errorf("error validating remote digest %q: %v", remoteDigest, err)
		}

		verifier, err := digest.NewDigestVerifier(remoteDigest)
		if err != nil {
			// There are not many ways this can go wrong: if it does, its
			// fatal. Likley, the cause would be poor validation of the
			// incoming reference.
			return "", nil, false, fmt.Errorf("error creating verifier for remote digest %q: %v", remoteDigest, err)
		}

		if _, err := verifier.Write(payload); err != nil {
			return "", nil, false, fmt.Errorf("error writing payload to remote digest verifier (verifier target %q): %v", remoteDigest, err)
		}

		if !verifier.Verified() {
			return "", nil, false, fmt.Errorf("verification against remote digest %q failed", remoteDigest)
		}
	}

	var manifest registry.ManifestData
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return "", nil, false, fmt.Errorf("error unmarshalling manifest: %s", err)
	}
	if manifest.SchemaVersion != 1 {
		return "", nil, false, fmt.Errorf("unsupported schema version: %d", manifest.SchemaVersion)
	}

	// validate the contents of the manifest
	if err := validateManifest(&manifest); err != nil {
		return "", nil, false, err
	}

	var verified bool
	for _, key := range keys {
		namespace := manifest.Name
		if namespace[0] != '/' {
			namespace = "/" + namespace
		}
		b, err := key.MarshalJSON()
		if err != nil {
			return "", nil, false, fmt.Errorf("error marshalling public key: %s", err)
		}
		// Check key has read/write permission (0x03)
		v, err := s.trustService.CheckKey(namespace, b, 0x03)
		if err != nil {
			vErr, ok := err.(trust.NotVerifiedError)
			if !ok {
				return "", nil, false, fmt.Errorf("error running key check: %s", err)
			}
			logrus.Debugf("Key check result: %v", vErr)
		}
		verified = v
		if verified {
			logrus.Debug("Key check result: verified")
		}
	}
	return localDigest, &manifest, verified, nil
}

func validateManifest(manifest *registry.ManifestData) error {
	if len(manifest.FSLayers) != len(manifest.History) {
		return fmt.Errorf("length of history not equal to number of layers")
	}

	if len(manifest.FSLayers) == 0 {
		return fmt.Errorf("no FSLayers in manifest")
	}

	return nil
}
