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

package tlog

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag/conv"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	v1 "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	rekortilespb "github.com/sigstore/rekor-tiles/v2/pkg/generated/protobuf"
	"github.com/sigstore/rekor-tiles/v2/pkg/note"
	typesverifier "github.com/sigstore/rekor-tiles/v2/pkg/types/verifier"
	"github.com/sigstore/rekor-tiles/v2/pkg/verify"
	"github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sigstore/rekor/pkg/types"
	dsse_v001 "github.com/sigstore/rekor/pkg/types/dsse/v0.0.1"
	hashedrekord_v001 "github.com/sigstore/rekor/pkg/types/hashedrekord/v0.0.1"
	intoto_v002 "github.com/sigstore/rekor/pkg/types/intoto/v0.0.2"
	rekorVerify "github.com/sigstore/rekor/pkg/verify"
	"github.com/sigstore/sigstore/pkg/signature"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/sigstore/sigstore-go/pkg/root"
)

type Entry struct {
	kind                 string
	version              string
	rekorV1Entry         types.EntryImpl
	rekorV2Entry         *rekortilespb.Entry
	signedEntryTimestamp []byte
	tle                  *v1.TransparencyLogEntry
}

type RekorPayload struct {
	Body           interface{} `json:"body"`
	IntegratedTime int64       `json:"integratedTime"`
	LogIndex       int64       `json:"logIndex"`
	LogID          string      `json:"logID"` //nolint:tagliatelle
}

var ErrNilValue = errors.New("validation error: nil value in transaction log entry")
var ErrInvalidRekorV2Entry = errors.New("type error: object is not a Rekor v2 type, try parsing as Rekor v1")

// Deprecated: use NewTlogEntry. NewEntry only parses a Rekor v1 entry.
func NewEntry(body []byte, integratedTime int64, logIndex int64, logID []byte, signedEntryTimestamp []byte, inclusionProof *models.InclusionProof) (*Entry, error) {
	pe, err := models.UnmarshalProposedEntry(bytes.NewReader(body), runtime.JSONConsumer())
	if err != nil {
		return nil, err
	}
	rekorEntry, err := types.UnmarshalEntry(pe)
	if err != nil {
		return nil, err
	}

	entry := &Entry{
		rekorV1Entry: rekorEntry,
		tle: &v1.TransparencyLogEntry{
			LogIndex: logIndex,
			LogId: &protocommon.LogId{
				KeyId: logID,
			},
			IntegratedTime:    integratedTime,
			CanonicalizedBody: body,
		},
		kind:    pe.Kind(),
		version: rekorEntry.APIVersion(),
	}

	if len(signedEntryTimestamp) > 0 {
		entry.signedEntryTimestamp = signedEntryTimestamp
	}

	if inclusionProof != nil {
		hashes := make([][]byte, len(inclusionProof.Hashes))
		for i, s := range inclusionProof.Hashes {
			hashes[i], err = hex.DecodeString(s)
			if err != nil {
				return nil, err
			}
		}
		rootHashDec, err := hex.DecodeString(*inclusionProof.RootHash)
		if err != nil {
			return nil, err
		}
		entry.tle.InclusionProof = &v1.InclusionProof{
			LogIndex: logIndex,
			RootHash: rootHashDec,
			TreeSize: *inclusionProof.TreeSize,
			Hashes:   hashes,
			Checkpoint: &v1.Checkpoint{
				Envelope: *inclusionProof.Checkpoint,
			},
		}
	}

	return entry, nil
}

func NewTlogEntry(tle *v1.TransparencyLogEntry) (*Entry, error) {
	var rekorV2Entry *rekortilespb.Entry
	var rekorV1Entry types.EntryImpl
	var err error

	body := tle.CanonicalizedBody
	rekorV2Entry, err = unmarshalRekorV2Entry(body)
	if err != nil {
		rekorV1Entry, err = unmarshalRekorV1Entry(body)
		if err != nil {
			return nil, fmt.Errorf("entry body is not a recognizable Rekor v1 or Rekor v2 type: %w", err)
		}
	}

	entry := &Entry{
		rekorV1Entry: rekorV1Entry,
		rekorV2Entry: rekorV2Entry,
		kind:         tle.KindVersion.Kind,
		version:      tle.KindVersion.Version,
	}

	signedEntryTimestamp := []byte{}
	if tle.InclusionPromise != nil && tle.InclusionPromise.SignedEntryTimestamp != nil {
		signedEntryTimestamp = tle.InclusionPromise.SignedEntryTimestamp
	}
	if len(signedEntryTimestamp) > 0 {
		entry.signedEntryTimestamp = signedEntryTimestamp
	}
	entry.tle = tle

	return entry, nil
}

func ParseTransparencyLogEntry(tle *v1.TransparencyLogEntry) (*Entry, error) {
	if tle == nil {
		return nil, ErrNilValue
	}
	if tle.CanonicalizedBody == nil ||
		tle.LogIndex < 0 ||
		tle.LogId == nil ||
		tle.LogId.KeyId == nil ||
		tle.KindVersion == nil {
		return nil, ErrNilValue
	}

	if tle.InclusionProof != nil {
		if tle.InclusionProof.Checkpoint == nil {
			return nil, fmt.Errorf("inclusion proof missing required checkpoint")
		}
		if tle.InclusionProof.Checkpoint.Envelope == "" {
			return nil, fmt.Errorf("inclusion proof checkpoint empty")
		}
	}

	entry, err := NewTlogEntry(tle)
	if err != nil {
		return nil, err
	}
	if entry.kind != tle.KindVersion.Kind || entry.version != tle.KindVersion.Version {
		return nil, fmt.Errorf("kind and version mismatch: %s/%s != %s/%s", entry.kind, entry.version, tle.KindVersion.Kind, tle.KindVersion.Version)
	}
	return entry, nil
}

// Deprecated: use ParseTransparencyLogEntry. ParseEntry only parses Rekor v1 type entries.
// ParseEntry decodes the entry bytes to a specific entry type (types.EntryImpl).
func ParseEntry(protoEntry *v1.TransparencyLogEntry) (entry *Entry, err error) {
	if protoEntry == nil ||
		protoEntry.CanonicalizedBody == nil ||
		protoEntry.IntegratedTime == 0 ||
		protoEntry.LogIndex < 0 ||
		protoEntry.LogId == nil ||
		protoEntry.LogId.KeyId == nil ||
		protoEntry.KindVersion == nil {
		return nil, ErrNilValue
	}

	signedEntryTimestamp := []byte{}
	if protoEntry.InclusionPromise != nil && protoEntry.InclusionPromise.SignedEntryTimestamp != nil {
		signedEntryTimestamp = protoEntry.InclusionPromise.SignedEntryTimestamp
	}

	var inclusionProof *models.InclusionProof

	if protoEntry.InclusionProof != nil {
		var hashes []string

		for _, v := range protoEntry.InclusionProof.Hashes {
			hashes = append(hashes, hex.EncodeToString(v))
		}

		rootHash := hex.EncodeToString(protoEntry.InclusionProof.RootHash)

		if protoEntry.InclusionProof.Checkpoint == nil {
			return nil, fmt.Errorf("inclusion proof missing required checkpoint")
		}
		if protoEntry.InclusionProof.Checkpoint.Envelope == "" {
			return nil, fmt.Errorf("inclusion proof checkpoint empty")
		}

		inclusionProof = &models.InclusionProof{
			LogIndex:   conv.Pointer(protoEntry.InclusionProof.LogIndex),
			RootHash:   &rootHash,
			TreeSize:   conv.Pointer(protoEntry.InclusionProof.TreeSize),
			Hashes:     hashes,
			Checkpoint: conv.Pointer(protoEntry.InclusionProof.Checkpoint.Envelope),
		}
	}

	entry, err = NewEntry(protoEntry.CanonicalizedBody, protoEntry.IntegratedTime, protoEntry.LogIndex, protoEntry.LogId.KeyId, signedEntryTimestamp, inclusionProof)
	if err != nil {
		return nil, err
	}

	if entry.kind != protoEntry.KindVersion.Kind || entry.version != protoEntry.KindVersion.Version {
		return nil, fmt.Errorf("kind and version mismatch: %s/%s != %s/%s", entry.kind, entry.version, protoEntry.KindVersion.Kind, protoEntry.KindVersion.Version)
	}
	entry.tle = protoEntry

	return entry, nil
}

func ValidateEntry(entry *Entry) error {
	if entry.rekorV1Entry != nil {
		switch e := entry.rekorV1Entry.(type) {
		case *dsse_v001.V001Entry:
			err := e.DSSEObj.Validate(strfmt.Default)
			if err != nil {
				return err
			}
		case *hashedrekord_v001.V001Entry:
			err := e.HashedRekordObj.Validate(strfmt.Default)
			if err != nil {
				return err
			}
		case *intoto_v002.V002Entry:
			err := e.IntotoObj.Validate(strfmt.Default)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported entry type: %T", e)
		}
	}
	if entry.rekorV2Entry != nil {
		switch e := entry.rekorV2Entry.GetSpec().GetSpec().(type) {
		case *rekortilespb.Spec_HashedRekordV002:
			err := validateHashedRekordV002Entry(e.HashedRekordV002)
			if err != nil {
				return err
			}
		case *rekortilespb.Spec_DsseV002:
			err := validateDSSEV002Entry(e.DsseV002)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func validateHashedRekordV002Entry(hr *rekortilespb.HashedRekordLogEntryV002) error {
	if hr.GetSignature() == nil || len(hr.GetSignature().GetContent()) == 0 {
		return fmt.Errorf("missing signature")
	}
	if hr.GetSignature().GetVerifier() == nil {
		return fmt.Errorf("missing verifier")
	}
	if hr.GetData() == nil {
		return fmt.Errorf("missing digest")
	}
	return typesverifier.Validate(hr.GetSignature().GetVerifier())
}

func validateDSSEV002Entry(d *rekortilespb.DSSELogEntryV002) error {
	if d.GetPayloadHash() == nil {
		return fmt.Errorf("missing payload")
	}
	if len(d.GetSignatures()) == 0 {
		return fmt.Errorf("missing signatures")
	}
	return typesverifier.Validate(d.GetSignatures()[0].GetVerifier())
}

func (entry *Entry) IntegratedTime() time.Time {
	if entry.tle.IntegratedTime == 0 {
		return time.Time{}
	}
	return time.Unix(entry.tle.IntegratedTime, 0)
}

func (entry *Entry) Signature() []byte {
	if entry.rekorV1Entry != nil {
		switch e := entry.rekorV1Entry.(type) {
		case *dsse_v001.V001Entry:
			sigBytes, err := base64.StdEncoding.DecodeString(*e.DSSEObj.Signatures[0].Signature)
			if err != nil {
				return []byte{}
			}
			return sigBytes
		case *hashedrekord_v001.V001Entry:
			return e.HashedRekordObj.Signature.Content
		case *intoto_v002.V002Entry:
			sigBytes, err := base64.StdEncoding.DecodeString(string(*e.IntotoObj.Content.Envelope.Signatures[0].Sig))
			if err != nil {
				return []byte{}
			}
			return sigBytes
		}
	}
	if entry.rekorV2Entry != nil {
		switch e := entry.rekorV2Entry.GetSpec().GetSpec().(type) {
		case *rekortilespb.Spec_HashedRekordV002:
			return e.HashedRekordV002.GetSignature().GetContent()
		case *rekortilespb.Spec_DsseV002:
			return e.DsseV002.GetSignatures()[0].GetContent()
		}
	}

	return []byte{}
}

func (entry *Entry) PublicKey() any {
	var pk any
	var certBytes []byte

	if entry.rekorV1Entry != nil {
		var pemString []byte
		switch e := entry.rekorV1Entry.(type) {
		case *dsse_v001.V001Entry:
			pemString = []byte(*e.DSSEObj.Signatures[0].Verifier)
		case *hashedrekord_v001.V001Entry:
			pemString = []byte(e.HashedRekordObj.Signature.PublicKey.Content)
		case *intoto_v002.V002Entry:
			pemString = []byte(*e.IntotoObj.Content.Envelope.Signatures[0].PublicKey)
		}
		certBlock, _ := pem.Decode(pemString)
		certBytes = certBlock.Bytes
	} else if entry.rekorV2Entry != nil {
		var verifier *rekortilespb.Verifier
		switch e := entry.rekorV2Entry.GetSpec().GetSpec().(type) {
		case *rekortilespb.Spec_HashedRekordV002:
			verifier = e.HashedRekordV002.GetSignature().GetVerifier()
		case *rekortilespb.Spec_DsseV002:
			verifier = e.DsseV002.GetSignatures()[0].GetVerifier()
		}
		switch verifier.Verifier.(type) {
		case *rekortilespb.Verifier_PublicKey:
			certBytes = verifier.GetPublicKey().GetRawBytes()
		case *rekortilespb.Verifier_X509Certificate:
			certBytes = verifier.GetX509Certificate().GetRawBytes()
		}
	}

	var err error

	pk, err = x509.ParseCertificate(certBytes)
	if err != nil {
		pk, err = x509.ParsePKIXPublicKey(certBytes)
		if err != nil {
			return nil
		}
	}

	return pk
}

func (entry *Entry) LogKeyID() string {
	return string(entry.tle.GetLogId().GetKeyId())
}

func (entry *Entry) LogIndex() int64 {
	return entry.tle.GetLogIndex()
}

func (entry *Entry) Body() any {
	return base64.StdEncoding.EncodeToString(entry.tle.CanonicalizedBody)
}

func (entry *Entry) HasInclusionPromise() bool {
	return entry.signedEntryTimestamp != nil
}

func (entry *Entry) HasInclusionProof() bool {
	return entry.tle.InclusionProof != nil
}

func (entry *Entry) TransparencyLogEntry() *v1.TransparencyLogEntry {
	return entry.tle
}

// VerifyInclusion verifies a Rekor v1-style checkpoint and the entry's inclusion in the Rekor v1 log.
func VerifyInclusion(entry *Entry, verifier signature.Verifier) error {
	hashes := make([]string, len(entry.tle.InclusionProof.Hashes))
	for i, b := range entry.tle.InclusionProof.Hashes {
		hashes[i] = hex.EncodeToString(b)
	}
	rootHash := hex.EncodeToString(entry.tle.GetInclusionProof().GetRootHash())
	logEntry := models.LogEntryAnon{
		IntegratedTime: &entry.tle.IntegratedTime,
		LogID:          conv.Pointer(string(entry.tle.GetLogId().KeyId)),
		LogIndex:       conv.Pointer(entry.tle.GetInclusionProof().GetLogIndex()),
		Body:           base64.StdEncoding.EncodeToString(entry.tle.GetCanonicalizedBody()),
		Verification: &models.LogEntryAnonVerification{
			InclusionProof: &models.InclusionProof{
				Checkpoint: conv.Pointer(entry.tle.GetInclusionProof().GetCheckpoint().GetEnvelope()),
				Hashes:     hashes,
				LogIndex:   conv.Pointer(entry.tle.GetInclusionProof().GetLogIndex()),
				RootHash:   &rootHash,
				TreeSize:   conv.Pointer(entry.tle.GetInclusionProof().GetTreeSize()),
			},
			SignedEntryTimestamp: strfmt.Base64(entry.signedEntryTimestamp),
		},
	}
	err := rekorVerify.VerifyInclusion(context.Background(), &logEntry)
	if err != nil {
		return err
	}

	err = rekorVerify.VerifyCheckpointSignature(&logEntry, verifier)
	if err != nil {
		return err
	}

	return nil
}

// VerifyCheckpointAndInclusion verifies a checkpoint and the entry's inclusion in the transparency log.
// This function is compatible with Rekor v1 and Rekor v2.
func VerifyCheckpointAndInclusion(entry *Entry, verifier signature.Verifier, origin string) error {
	noteVerifier, err := note.NewNoteVerifier(origin, verifier)
	if err != nil {
		return fmt.Errorf("loading note verifier: %w", err)
	}
	err = verify.VerifyLogEntry(entry.TransparencyLogEntry(), noteVerifier)
	if err != nil {
		return fmt.Errorf("verifying log entry: %w", err)
	}

	return nil
}

func VerifySET(entry *Entry, verifiers map[string]*root.TransparencyLog) error {
	if entry.rekorV1Entry == nil {
		return fmt.Errorf("can only verify SET for Rekor v1 entry")
	}
	rekorPayload := RekorPayload{
		Body:           entry.Body(),
		IntegratedTime: entry.tle.IntegratedTime,
		LogIndex:       entry.LogIndex(),
		LogID:          hex.EncodeToString([]byte(entry.LogKeyID())),
	}

	verifier, ok := verifiers[hex.EncodeToString([]byte(entry.LogKeyID()))]
	if !ok {
		return errors.New("rekor log public key not found for payload")
	}
	if verifier.ValidityPeriodStart.IsZero() {
		return errors.New("rekor validity period start time not set")
	}
	if (verifier.ValidityPeriodStart.After(entry.IntegratedTime())) ||
		(!verifier.ValidityPeriodEnd.IsZero() && verifier.ValidityPeriodEnd.Before(entry.IntegratedTime())) {
		return errors.New("rekor log public key not valid at payload integrated time")
	}

	contents, err := json.Marshal(rekorPayload)
	if err != nil {
		return fmt.Errorf("marshaling: %w", err)
	}
	canonicalized, err := jsoncanonicalizer.Transform(contents)
	if err != nil {
		return fmt.Errorf("canonicalizing: %w", err)
	}

	hash := sha256.Sum256(canonicalized)
	if ecdsaPublicKey, ok := verifier.PublicKey.(*ecdsa.PublicKey); !ok {
		return fmt.Errorf("unsupported public key type: %T", verifier.PublicKey)
	} else if !ecdsa.VerifyASN1(ecdsaPublicKey, hash[:], entry.signedEntryTimestamp) {
		return errors.New("unable to verify SET")
	}
	return nil
}

func unmarshalRekorV1Entry(body []byte) (types.EntryImpl, error) {
	pe, err := models.UnmarshalProposedEntry(bytes.NewReader(body), runtime.JSONConsumer())
	if err != nil {
		return nil, err
	}
	rekorEntry, err := types.UnmarshalEntry(pe)
	if err != nil {
		return nil, err
	}
	return rekorEntry, nil
}

func unmarshalRekorV2Entry(body []byte) (*rekortilespb.Entry, error) {
	logEntryBody := rekortilespb.Entry{}
	err := protojson.Unmarshal(body, &logEntryBody)
	if err != nil {
		return nil, ErrInvalidRekorV2Entry
	}
	return &logEntryBody, nil
}
