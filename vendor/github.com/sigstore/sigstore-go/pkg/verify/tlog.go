// Copyright 2023 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package verify

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	rekortilespb "github.com/sigstore/rekor-tiles/v2/pkg/generated/protobuf"
	"github.com/sigstore/rekor-tiles/v2/pkg/note"
	"github.com/sigstore/rekor-tiles/v2/pkg/types/hashedrekord"
	rekorVerify "github.com/sigstore/rekor-tiles/v2/pkg/verify"
	"github.com/sigstore/sigstore-go/internal/limits"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tlog"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

// VerifyTlogEntry verifies that the given entity has been logged
// in the transparency log and that the log entry is valid.
//
// The threshold parameter is the number of unique transparency log entries
// that must be verified.
func VerifyTlogEntry(entity SignedEntity, trustedMaterial root.TrustedMaterial, logThreshold int, trustIntegratedTime bool) ([]root.Timestamp, error) { //nolint:revive
	entries, err := entity.TlogEntries()
	if err != nil {
		return nil, err
	}

	// limit the number of tlog entries to prevent DoS
	if len(entries) > limits.MaxAllowedTlogEntries {
		return nil, fmt.Errorf("too many tlog entries: %d > %d", len(entries), limits.MaxAllowedTlogEntries)
	}

	// disallow duplicate entries, as a malicious actor could use duplicates to bypass the threshold
	for i := range entries {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].LogKeyID() == entries[j].LogKeyID() && entries[i].LogIndex() == entries[j].LogIndex() {
				return nil, errors.New("duplicate tlog entries found")
			}
		}
	}

	sigContent, err := entity.SignatureContent()
	if err != nil {
		return nil, err
	}

	entitySignature := sigContent.Signature()

	verificationContent, err := entity.VerificationContent()
	if err != nil {
		return nil, err
	}

	var verifiedTimestamps []root.Timestamp
	verifiedLogIDsMap := make(map[string]bool)
	hasTimestampMap := make(map[string]bool)

	for _, entry := range entries {
		err := tlog.ValidateEntry(entry)
		if err != nil {
			return nil, err
		}

		rekorLogs := trustedMaterial.RekorLogs()
		keyID := entry.LogKeyID()
		hex64Key := hex.EncodeToString([]byte(keyID))
		tlogVerifier, ok := trustedMaterial.RekorLogs()[hex64Key]
		if !ok {
			// skip entries the trust root cannot verify
			continue
		}

		if !entry.HasInclusionPromise() && !entry.HasInclusionProof() {
			return nil, fmt.Errorf("entry must contain an inclusion proof and/or promise")
		}
		if entry.IsRekorV2() && !entry.HasInclusionProof() {
			return nil, fmt.Errorf("rekor v2 entries must have an inclusion proof")
		}
		if entry.HasInclusionPromise() {
			err = tlog.VerifySET(entry, rekorLogs)
			if err != nil {
				// skip entries the trust root cannot verify
				continue
			}
		}
		if entry.HasInclusionProof() {
			verifier, err := getVerifier(tlogVerifier.PublicKey, tlogVerifier.SignatureHashFunc)
			if err != nil {
				return nil, err
			}

			if hasRekorV1STH(entry) {
				err = tlog.VerifyInclusion(entry, *verifier)
				if err != nil {
					return nil, err
				}
			} else {
				if tlogVerifier.BaseURL == "" {
					return nil, fmt.Errorf("cannot verify Rekor v2 entry without baseUrl in transparency log's trusted root")
				}
				u, err := url.Parse(tlogVerifier.BaseURL)
				if err != nil {
					return nil, err
				}
				noteVerifier, err := note.NewNoteVerifier(u.Hostname(), *verifier)
				if err != nil {
					return nil, fmt.Errorf("loading note verifier: %w", err)
				}
				entryHash, err := reconstructV2EntryHash(sigContent, verificationContent, trustedMaterial, entitySignature)
				if err != nil {
					return nil, err
				}
				if err := rekorVerify.VerifyLogEntryWithHash(entry.TransparencyLogEntry(), noteVerifier, entryHash); err != nil {
					return nil, fmt.Errorf("verifying log entry: %w", err)
				}
			}
			// DO NOT use timestamp with only an inclusion proof, because it is not signed metadata
		}

		// Rekor v1 only: enforce bundle ↔ entry equality field-by-field.
		// Rekor v2 skips this because the reconstructed hash already commits
		// to the signature, public key, and digest from the bundle.
		if !entry.IsRekorV2() {
			if !bytes.Equal(entry.Signature(), entitySignature) {
				return nil, errors.New("transparency log signature does not match")
			}

			if !verificationContent.CompareKey(entry.PublicKey(), trustedMaterial) {
				return nil, errors.New("transparency log certificate does not match")
			}

			switch {
			case sigContent.MessageSignatureContent() != nil:
				msgSig := sigContent.MessageSignatureContent()
				entityDigest := msgSig.Digest()
				entityAlgo := msgSig.DigestAlgorithm()

				entryDigest, entryAlgo, ok := entry.GetHashedRekordDigest()
				if !ok {
					return nil, errors.New("transparency log entry is not a hashedrekord or missing digest")
				}
				entityHashFunc, err := algStringToHashFunc(entityAlgo)
				if err != nil {
					return nil, err
				}
				entryHashFunc, err := algStringToHashFunc(entryAlgo)
				if err != nil {
					return nil, err
				}
				if entityHashFunc != entryHashFunc {
					return nil, fmt.Errorf("transparency log hashedrekord entry digest algorithm mismatch: %s != %s", entityAlgo, entryAlgo)
				}
				if !bytes.Equal(entityDigest, entryDigest) {
					return nil, fmt.Errorf("transparency log hashedrekord entry digest %s does not match artifact %s", hex.EncodeToString(entryDigest), hex.EncodeToString(entityDigest))
				}
			case sigContent.EnvelopeContent() != nil:
				env := sigContent.EnvelopeContent().RawEnvelope()
				if env == nil {
					return nil, errors.New("bundle envelope is missing")
				}
				payloadBytes, err := base64.StdEncoding.DecodeString(env.Payload)
				if err != nil {
					return nil, fmt.Errorf("failed to decode envelope payload: %w", err)
				}
				payloadHash := sha256.Sum256(payloadBytes)
				entryDigest, ok := entry.GetDssePayloadHash()
				if !ok {
					return nil, errors.New("transparency log rekor v1 entry is not a dsse_v001 or intoto_v002 entry")
				}
				if !bytes.Equal(payloadHash[:], entryDigest) {
					return nil, fmt.Errorf("transparency log dsse/intoto entry payload hash %s does not match envelope payload hash %s", hex.EncodeToString(payloadHash[:]), hex.EncodeToString(entryDigest))
				}
			default:
				return nil, errors.New("bundle must contain either a message signature or an envelope")
			}
		}

		// Check tlog entry time against bundle certificates
		if !entry.IntegratedTime().IsZero() {
			if !verificationContent.ValidAtTime(entry.IntegratedTime(), trustedMaterial) {
				return nil, errors.New("integrated time outside certificate validity")
			}
		}

		// successful log entry verification
		verifiedLogIDsMap[keyID] = true
		if trustIntegratedTime && entry.HasInclusionPromise() && !hasTimestampMap[keyID] {
			hasTimestampMap[keyID] = true
			verifiedTimestamps = append(verifiedTimestamps, root.Timestamp{Time: entry.IntegratedTime(), URI: tlogVerifier.BaseURL})
		}
	}

	if len(verifiedLogIDsMap) < logThreshold {
		return nil, fmt.Errorf("not enough verified log entries from transparency log: %d < %d", len(verifiedLogIDsMap), logThreshold)
	}

	return verifiedTimestamps, nil
}

// reconstructV2EntryHash rebuilds a Rekor v2 entry hash from bundle content.
func reconstructV2EntryHash(sigContent SignatureContent, verificationContent VerificationContent, trustedMaterial root.TrustedMaterial, entitySignature []byte) ([]byte, error) {
	var (
		pubKey        crypto.PublicKey
		verifierProto *rekortilespb.Verifier
	)
	if leafCert := verificationContent.Certificate(); leafCert != nil {
		pubKey = leafCert.PublicKey
		verifierProto = &rekortilespb.Verifier{
			Verifier: &rekortilespb.Verifier_X509Certificate{
				X509Certificate: &protocommon.X509Certificate{RawBytes: leafCert.Raw},
			},
		}
	} else if pkp := verificationContent.PublicKey(); pkp != nil {
		v, err := trustedMaterial.PublicKeyVerifier(pkp.Hint())
		if err != nil {
			return nil, fmt.Errorf("public key not found in trusted material: %w", err)
		}
		pk, err := v.PublicKey()
		if err != nil {
			return nil, fmt.Errorf("getting public key: %w", err)
		}
		pubKey = pk
		rawBytes, err := x509.MarshalPKIXPublicKey(pk)
		if err != nil {
			return nil, fmt.Errorf("marshaling public key: %w", err)
		}
		verifierProto = &rekortilespb.Verifier{
			Verifier: &rekortilespb.Verifier_PublicKey{
				PublicKey: &rekortilespb.PublicKey{RawBytes: rawBytes},
			},
		}
	} else {
		return nil, errors.New("verification content has neither certificate nor public key")
	}

	algDetails, err := signature.GetDefaultAlgorithmDetails(pubKey, options.WithED25519ph())
	if err != nil {
		return nil, fmt.Errorf("getting algorithm details from bundle key: %w", err)
	}
	verifierProto.KeyDetails = algDetails.GetSignatureAlgorithm()

	hf := algDetails.GetHashType()
	if hf == crypto.Hash(0) {
		return nil, errors.New("rekor v2 hashedrekord entries require a prehashing signature algorithm")
	}

	var digest []byte
	switch {
	case sigContent.MessageSignatureContent() != nil:
		digest = sigContent.MessageSignatureContent().Digest()
	case sigContent.EnvelopeContent() != nil:
		env := sigContent.EnvelopeContent().RawEnvelope()
		if env == nil {
			return nil, errors.New("bundle envelope is missing")
		}
		payloadBytes, err := base64.StdEncoding.DecodeString(env.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to decode envelope payload: %w", err)
		}
		hasher := hf.New()
		hasher.Write(dsse.PAE(env.PayloadType, payloadBytes))
		digest = hasher.Sum(nil)
	default:
		return nil, errors.New("bundle must contain either a message signature or an envelope")
	}

	return hashedrekord.ToEntryHash(digest, &rekortilespb.Signature{
		Content:  entitySignature,
		Verifier: verifierProto,
	})
}

func getVerifier(publicKey crypto.PublicKey, hashFunc crypto.Hash) (*signature.Verifier, error) {
	verifier, err := signature.LoadVerifier(publicKey, hashFunc)
	if err != nil {
		return nil, err
	}

	return &verifier, nil
}

// TODO: remove this deprecated function before 2.0

// Deprecated: use VerifyTlogEntry instead
func VerifyArtifactTransparencyLog(entity SignedEntity, trustedMaterial root.TrustedMaterial, logThreshold int, trustIntegratedTime bool) ([]root.Timestamp, error) { //nolint:revive
	return VerifyTlogEntry(entity, trustedMaterial, logThreshold, trustIntegratedTime)
}

var treeIDSuffixRegex = regexp.MustCompile(".* - [0-9]+$")

// hasRekorV1STH checks if the checkpoint has a Rekor v1-style Signed Tree Head
// which contains a numeric Tree ID as part of its checkpoint origin.
func hasRekorV1STH(entry *tlog.Entry) bool {
	tle := entry.TransparencyLogEntry()
	checkpointBody := tle.GetInclusionProof().GetCheckpoint().GetEnvelope()
	checkpointLines := strings.Split(checkpointBody, "\n")
	if len(checkpointLines) < 4 {
		return false
	}
	return treeIDSuffixRegex.MatchString(checkpointLines[0])
}
