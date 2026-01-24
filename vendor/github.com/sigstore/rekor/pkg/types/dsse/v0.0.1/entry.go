//
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

package dsse

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag/conv"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sigstore/rekor/pkg/internal/log"
	pkitypes "github.com/sigstore/rekor/pkg/pki/pkitypes"
	"github.com/sigstore/rekor/pkg/pki/x509"
	"github.com/sigstore/rekor/pkg/types"
	dsseType "github.com/sigstore/rekor/pkg/types/dsse"
	"github.com/sigstore/sigstore/pkg/signature"
	sigdsse "github.com/sigstore/sigstore/pkg/signature/dsse"
)

const (
	APIVERSION = "0.0.1"
)

func init() {
	if err := dsseType.VersionMap.SetEntryFactory(APIVERSION, NewEntry); err != nil {
		log.Logger.Panic(err)
	}
}

type V001Entry struct {
	DSSEObj models.DSSEV001Schema
	env     *dsse.Envelope
}

func (v V001Entry) APIVersion() string {
	return APIVERSION
}

func NewEntry() types.EntryImpl {
	return &V001Entry{}
}

// IndexKeys computes the list of keys that should map back to this entry.
// It should *never* reference v.DSSEObj.ProposedContent as those values would only
// be present at the time of insertion
func (v V001Entry) IndexKeys() ([]string, error) {
	var result []string

	for _, sig := range v.DSSEObj.Signatures {
		if sig == nil || sig.Verifier == nil {
			return result, errors.New("missing or malformed public key")
		}
		keyObj, err := x509.NewPublicKey(bytes.NewReader(*sig.Verifier))
		if err != nil {
			return result, err
		}

		canonKey, err := keyObj.CanonicalValue()
		if err != nil {
			return result, fmt.Errorf("could not canonicalize key: %w", err)
		}

		keyHash := sha256.Sum256(canonKey)
		result = append(result, "sha256:"+hex.EncodeToString(keyHash[:]))

		result = append(result, keyObj.Subjects()...)
	}

	if v.DSSEObj.PayloadHash != nil {
		payloadHashKey := strings.ToLower(fmt.Sprintf("%s:%s", *v.DSSEObj.PayloadHash.Algorithm, *v.DSSEObj.PayloadHash.Value))
		result = append(result, payloadHashKey)
	}

	if v.DSSEObj.EnvelopeHash != nil {
		envelopeHashKey := strings.ToLower(fmt.Sprintf("%s:%s", *v.DSSEObj.EnvelopeHash.Algorithm, *v.DSSEObj.EnvelopeHash.Value))
		result = append(result, envelopeHashKey)
	}

	if v.env == nil {
		log.Logger.Info("DSSEObj content or DSSE envelope is nil, returning partial set of keys")
		return result, nil
	}

	switch v.env.PayloadType {
	case in_toto.PayloadType:

		if v.env.Payload == "" {
			log.Logger.Info("DSSEObj DSSE payload is empty")
			return result, nil
		}
		decodedPayload, err := v.env.DecodeB64Payload()
		if err != nil {
			return result, fmt.Errorf("could not decode envelope payload: %w", err)
		}
		statement, err := parseStatement(decodedPayload)
		if err != nil {
			return result, err
		}
		for _, s := range statement.Subject {
			for alg, ds := range s.Digest {
				result = append(result, alg+":"+ds)
			}
		}
		// Not all in-toto statements will contain a SLSA provenance predicate.
		// See https://github.com/in-toto/attestation/blob/main/spec/README.md#predicate
		// for other predicates.
		if predicate, err := parseSlsaPredicate(decodedPayload); err == nil {
			if predicate.Predicate.Materials != nil {
				for _, s := range predicate.Predicate.Materials {
					for alg, ds := range s.Digest {
						result = append(result, alg+":"+ds)
					}
				}
			}
		}
	default:
		log.Logger.Infof("Unknown DSSE envelope payloadType: %s", v.env.PayloadType)
	}
	return result, nil
}

func parseStatement(p []byte) (*in_toto.Statement, error) {
	ps := in_toto.Statement{}
	if err := json.Unmarshal(p, &ps); err != nil {
		return nil, err
	}
	return &ps, nil
}

func parseSlsaPredicate(p []byte) (*in_toto.ProvenanceStatement, error) {
	predicate := in_toto.ProvenanceStatement{}
	if err := json.Unmarshal(p, &predicate); err != nil {
		return nil, err
	}
	return &predicate, nil
}

// DecodeEntry performs direct decode into the provided output pointer
// without mutating the receiver on error.
func DecodeEntry(input any, output *models.DSSEV001Schema) error {
	if output == nil {
		return fmt.Errorf("nil output *models.DSSEV001Schema")
	}
	var m models.DSSEV001Schema
	// Single switch with map fast path
	switch data := input.(type) {
	case map[string]any:
		mm := data
		if pcRaw, ok := mm["proposedContent"].(map[string]any); ok {
			m.ProposedContent = &models.DSSEV001SchemaProposedContent{}
			if env, ok := pcRaw["envelope"].(string); ok {
				m.ProposedContent.Envelope = &env
			}
			if vsIF, ok := pcRaw["verifiers"].([]any); ok {
				m.ProposedContent.Verifiers = make([]strfmt.Base64, 0, len(vsIF))
				for _, it := range vsIF {
					if s, ok := it.(string); ok && s != "" {
						outb := make([]byte, base64.StdEncoding.DecodedLen(len(s)))
						n, err := base64.StdEncoding.Decode(outb, []byte(s))
						if err != nil {
							return fmt.Errorf("failed parsing base64 data for verifier: %w", err)
						}
						m.ProposedContent.Verifiers = append(m.ProposedContent.Verifiers, strfmt.Base64(outb[:n]))
					}
				}
			} else if vsStr, ok := pcRaw["verifiers"].([]string); ok {
				m.ProposedContent.Verifiers = make([]strfmt.Base64, 0, len(vsStr))
				for _, s := range vsStr {
					if s == "" {
						continue
					}
					outb := make([]byte, base64.StdEncoding.DecodedLen(len(s)))
					n, err := base64.StdEncoding.Decode(outb, []byte(s))
					if err != nil {
						return fmt.Errorf("failed parsing base64 data for verifier: %w", err)
					}
					m.ProposedContent.Verifiers = append(m.ProposedContent.Verifiers, strfmt.Base64(outb[:n]))
				}
			}
		}
		if sigs, ok := mm["signatures"].([]any); ok {
			m.Signatures = make([]*models.DSSEV001SchemaSignaturesItems0, 0, len(sigs))
			for _, s := range sigs {
				if sm, ok := s.(map[string]any); ok {
					item := &models.DSSEV001SchemaSignaturesItems0{}
					if sig, ok := sm["signature"].(string); ok {
						item.Signature = &sig
					}
					if vr, ok := sm["verifier"].(string); ok && vr != "" {
						outb := make([]byte, base64.StdEncoding.DecodedLen(len(vr)))
						n, err := base64.StdEncoding.Decode(outb, []byte(vr))
						if err != nil {
							return fmt.Errorf("failed parsing base64 data for signature verifier: %w", err)
						}
						b := strfmt.Base64(outb[:n])
						item.Verifier = &b
					}
					m.Signatures = append(m.Signatures, item)
				}
			}
		}
		if eh, ok := mm["envelopeHash"].(map[string]any); ok {
			m.EnvelopeHash = &models.DSSEV001SchemaEnvelopeHash{}
			if alg, ok := eh["algorithm"].(string); ok {
				m.EnvelopeHash.Algorithm = &alg
			}
			if val, ok := eh["value"].(string); ok {
				m.EnvelopeHash.Value = &val
			}
		}
		if ph, ok := mm["payloadHash"].(map[string]any); ok {
			m.PayloadHash = &models.DSSEV001SchemaPayloadHash{}
			if alg, ok := ph["algorithm"].(string); ok {
				m.PayloadHash.Algorithm = &alg
			}
			if val, ok := ph["value"].(string); ok {
				m.PayloadHash.Value = &val
			}
		}
		*output = m
		return nil
	case *models.DSSEV001Schema:
		if data == nil {
			return fmt.Errorf("nil *models.DSSEV001Schema")
		}
		*output = *data
		return nil
	case models.DSSEV001Schema:
		*output = data
		return nil
	default:
		return fmt.Errorf("unsupported input type %T for DecodeEntry", input)
	}
}

func (v *V001Entry) Unmarshal(pe models.ProposedEntry) error {
	it, ok := pe.(*models.DSSE)
	if !ok {
		return errors.New("cannot unmarshal non DSSE v0.0.1 type")
	}

	dsseObj := &models.DSSEV001Schema{}

	if err := DecodeEntry(it.Spec, dsseObj); err != nil {
		return err
	}

	// field validation
	if err := dsseObj.Validate(strfmt.Default); err != nil {
		return err
	}

	// either we have just proposed content or the canonicalized fields
	if dsseObj.ProposedContent == nil {
		// then we need canonicalized fields, and all must be present (if present, they would have been validated in the above call to Validate())
		if dsseObj.EnvelopeHash == nil || dsseObj.PayloadHash == nil || len(dsseObj.Signatures) == 0 {
			return errors.New("either proposedContent or envelopeHash, payloadHash, and signatures must be present")
		}
		v.DSSEObj = *dsseObj
		return nil
	}
	// if we're here, then we're trying to propose a new entry so we check to ensure client's aren't setting server-side computed fields
	if dsseObj.EnvelopeHash != nil || dsseObj.PayloadHash != nil || len(dsseObj.Signatures) != 0 {
		return errors.New("either proposedContent or envelopeHash, payloadHash, and signatures must be present but not both")
	}

	env := &dsse.Envelope{}
	if dsseObj.ProposedContent.Envelope == nil {
		return errors.New("proposed content envelope is missing")
	}
	if err := json.Unmarshal([]byte(*dsseObj.ProposedContent.Envelope), env); err != nil {
		return err
	}

	if len(env.Signatures) == 0 {
		return errors.New("DSSE envelope must contain 1 or more signatures")
	}

	allPubKeyBytes := make([][]byte, 0)
	for _, publicKey := range dsseObj.ProposedContent.Verifiers {
		if publicKey == nil {
			return errors.New("an invalid null verifier was provided in ProposedContent")
		}

		allPubKeyBytes = append(allPubKeyBytes, publicKey)
	}

	sigToKeyMap, err := verifyEnvelope(allPubKeyBytes, env)
	if err != nil {
		return err
	}

	// we need to ensure we canonicalize the ordering of signatures
	sortedSigs := make([]string, 0, len(sigToKeyMap))
	for sig := range sigToKeyMap {
		sortedSigs = append(sortedSigs, sig)
	}
	sort.Strings(sortedSigs)

	for i, sig := range sortedSigs {
		key := sigToKeyMap[sig]
		canonicalizedKey, err := key.CanonicalValue()
		if err != nil {
			return err
		}
		b64CanonicalizedKey := strfmt.Base64(canonicalizedKey)

		dsseObj.Signatures = append(dsseObj.Signatures, &models.DSSEV001SchemaSignaturesItems0{
			Signature: &sortedSigs[i],
			Verifier:  &b64CanonicalizedKey,
		})
	}

	decodedPayload, err := env.DecodeB64Payload()
	if err != nil {
		// this shouldn't happen because failure would have occurred in verifyEnvelope call above
		return err
	}

	payloadHash := sha256.Sum256(decodedPayload)
	dsseObj.PayloadHash = &models.DSSEV001SchemaPayloadHash{
		Algorithm: conv.Pointer(models.DSSEV001SchemaPayloadHashAlgorithmSha256),
		Value:     conv.Pointer(hex.EncodeToString(payloadHash[:])),
	}

	envelopeHash := sha256.Sum256([]byte(*dsseObj.ProposedContent.Envelope))
	dsseObj.EnvelopeHash = &models.DSSEV001SchemaEnvelopeHash{
		Algorithm: conv.Pointer(models.DSSEV001SchemaEnvelopeHashAlgorithmSha256),
		Value:     conv.Pointer(hex.EncodeToString(envelopeHash[:])),
	}

	// we've gotten through all processing without error, now update the object we're unmarshalling into
	v.DSSEObj = *dsseObj
	v.env = env

	return nil
}

// Canonicalize returns a JSON representation of the entry to be persisted into the log. This
// will be further canonicalized by JSON Canonicalization Scheme (JCS) before being written.
//
// This function should not use v.DSSEObj.ProposedContent fields as they are client provided and
// should not be trusted; the other fields at the top level are only set server side.
func (v *V001Entry) Canonicalize(_ context.Context) ([]byte, error) {
	canonicalEntry := models.DSSEV001Schema{
		Signatures:      v.DSSEObj.Signatures,
		EnvelopeHash:    v.DSSEObj.EnvelopeHash,
		PayloadHash:     v.DSSEObj.PayloadHash,
		ProposedContent: nil, // this is explicitly done as we don't want to canonicalize the envelope
	}

	for _, s := range canonicalEntry.Signatures {
		if s == nil || s.Signature == nil {
			return nil, errors.New("canonical entry missing required signature")
		}
	}

	sort.Slice(canonicalEntry.Signatures, func(i, j int) bool {
		return *canonicalEntry.Signatures[i].Signature < *canonicalEntry.Signatures[j].Signature
	})

	itObj := models.DSSE{}
	itObj.APIVersion = conv.Pointer(APIVERSION)
	itObj.Spec = &canonicalEntry

	return json.Marshal(&itObj)
}

// AttestationKey and AttestationKeyValue are not implemented so the envelopes will not be persisted in Rekor

func (v V001Entry) CreateFromArtifactProperties(_ context.Context, props types.ArtifactProperties) (models.ProposedEntry, error) {
	returnVal := models.DSSE{}
	re := V001Entry{
		DSSEObj: models.DSSEV001Schema{
			ProposedContent: &models.DSSEV001SchemaProposedContent{},
		},
	}
	var err error
	artifactBytes := props.ArtifactBytes
	if artifactBytes == nil {
		if props.ArtifactPath == nil {
			return nil, errors.New("path to artifact file must be specified")
		}
		if props.ArtifactPath.IsAbs() {
			return nil, errors.New("dsse envelopes cannot be fetched over HTTP(S)")
		}
		artifactBytes, err = os.ReadFile(filepath.Clean(props.ArtifactPath.Path))
		if err != nil {
			return nil, err
		}
	}

	env := &dsse.Envelope{}
	if err := json.Unmarshal(artifactBytes, env); err != nil {
		return nil, fmt.Errorf("payload must be a valid DSSE envelope: %w", err)
	}

	allPubKeyBytes := make([][]byte, 0)
	if len(props.PublicKeyBytes) > 0 {
		allPubKeyBytes = append(allPubKeyBytes, props.PublicKeyBytes...)
	}

	if len(props.PublicKeyPaths) > 0 {
		for _, path := range props.PublicKeyPaths {
			if path.IsAbs() {
				return nil, errors.New("dsse public keys cannot be fetched over HTTP(S)")
			}

			publicKeyBytes, err := os.ReadFile(filepath.Clean(path.Path))
			if err != nil {
				return nil, fmt.Errorf("error reading public key file: %w", err)
			}

			allPubKeyBytes = append(allPubKeyBytes, publicKeyBytes)
		}
	}

	keysBySig, err := verifyEnvelope(allPubKeyBytes, env)
	if err != nil {
		return nil, err
	}
	for _, key := range keysBySig {
		canonicalKey, err := key.CanonicalValue()
		if err != nil {
			return nil, err
		}
		re.DSSEObj.ProposedContent.Verifiers = append(re.DSSEObj.ProposedContent.Verifiers, strfmt.Base64(canonicalKey))
	}
	re.DSSEObj.ProposedContent.Envelope = conv.Pointer(string(artifactBytes))

	returnVal.Spec = re.DSSEObj
	returnVal.APIVersion = conv.Pointer(re.APIVersion())

	return &returnVal, nil
}

// verifyEnvelope takes in an array of possible key bytes and attempts to parse them as x509 public keys.
// it then uses these to verify the envelope and makes sure that every signature on the envelope is verified.
// it returns a map of verifiers indexed by the signature the verifier corresponds to.
func verifyEnvelope(allPubKeyBytes [][]byte, env *dsse.Envelope) (map[string]*x509.PublicKey, error) {
	// generate a fake id for these keys so we can get back to the key bytes and match them to their corresponding signature
	verifierBySig := make(map[string]*x509.PublicKey)
	allSigs := make(map[string]struct{})
	for _, sig := range env.Signatures {
		allSigs[sig.Sig] = struct{}{}
	}

	for _, pubKeyBytes := range allPubKeyBytes {
		if len(allSigs) == 0 {
			break // if all signatures have been verified, do not attempt anymore
		}
		key, err := x509.NewPublicKey(bytes.NewReader(pubKeyBytes))
		if err != nil {
			return nil, fmt.Errorf("could not parse public key as x509: %w", err)
		}

		vfr, err := signature.LoadVerifier(key.CryptoPubKey(), crypto.SHA256)
		if err != nil {
			return nil, fmt.Errorf("could not load verifier: %w", err)
		}

		dsseVfr, err := dsse.NewEnvelopeVerifier(&sigdsse.VerifierAdapter{SignatureVerifier: vfr})
		if err != nil {
			return nil, fmt.Errorf("could not use public key as a dsse verifier: %w", err)
		}

		accepted, err := dsseVfr.Verify(context.Background(), env)
		if err != nil {
			return nil, fmt.Errorf("could not verify envelope: %w", err)
		}

		for _, accept := range accepted {
			delete(allSigs, accept.Sig.Sig)
			verifierBySig[accept.Sig.Sig] = key
		}
	}

	if len(allSigs) > 0 {
		return nil, errors.New("all signatures must have a key that verifies it")
	}

	return verifierBySig, nil
}

func (v V001Entry) Verifiers() ([]pkitypes.PublicKey, error) {
	if len(v.DSSEObj.Signatures) == 0 {
		return nil, errors.New("dsse v0.0.1 entry not initialized")
	}

	var keys []pkitypes.PublicKey
	for _, s := range v.DSSEObj.Signatures {
		key, err := x509.NewPublicKey(bytes.NewReader(*s.Verifier))
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (v V001Entry) ArtifactHash() (string, error) {
	if v.DSSEObj.PayloadHash == nil || v.DSSEObj.PayloadHash.Algorithm == nil || v.DSSEObj.PayloadHash.Value == nil {
		return "", errors.New("dsse v0.0.1 entry not initialized")
	}
	return strings.ToLower(fmt.Sprintf("%s:%s", *v.DSSEObj.PayloadHash.Algorithm, *v.DSSEObj.PayloadHash.Value)), nil
}

func (v V001Entry) Insertable() (bool, error) {
	if v.DSSEObj.ProposedContent == nil {
		return false, errors.New("missing proposed content")
	}
	if v.DSSEObj.ProposedContent.Envelope == nil || len(*v.DSSEObj.ProposedContent.Envelope) == 0 {
		return false, errors.New("missing proposed DSSE envelope")
	}
	if len(v.DSSEObj.ProposedContent.Verifiers) == 0 {
		return false, errors.New("missing proposed verifiers")
	}

	return true, nil
}
