//
// Copyright 2021 The Sigstore Authors.
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

package types

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"reflect"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/go-openapi/strfmt"
	"github.com/go-viper/mapstructure/v2"
	"github.com/sigstore/rekor/pkg/generated/models"
	pkitypes "github.com/sigstore/rekor/pkg/pki/pkitypes"
)

// EntryImpl specifies the behavior of a versioned type
type EntryImpl interface {
	APIVersion() string                               // the supported versions for this implementation
	IndexKeys() ([]string, error)                     // the keys that should be added to the external index for this entry
	Canonicalize(ctx context.Context) ([]byte, error) // marshal the canonical entry to be put into the tlog
	Unmarshal(e models.ProposedEntry) error           // unmarshal the abstract entry into the specific struct for this versioned type
	CreateFromArtifactProperties(context.Context, ArtifactProperties) (models.ProposedEntry, error)
	Verifiers() ([]pkitypes.PublicKey, error) // list of keys or certificates that can verify an entry's signature
	ArtifactHash() (string, error)            // hex-encoded artifact hash prefixed with hash name, e.g. sha256:abcdef
	Insertable() (bool, error)                // denotes whether the entry that was unmarshalled has the writeOnly fields required to validate and insert into the log
}

// EntryWithAttestationImpl specifies the behavior of a versioned type that also stores attestations
type EntryWithAttestationImpl interface {
	EntryImpl
	AttestationKey() string                // returns the key used to look up the attestation from storage (should be sha256:digest)
	AttestationKeyValue() (string, []byte) // returns the key to be used when storing the attestation as well as the attestation itself
}

// ProposedEntryIterator is an iterator over a list of proposed entries
type ProposedEntryIterator interface {
	models.ProposedEntry
	HasNext() bool
	Get() models.ProposedEntry
	GetNext() models.ProposedEntry
}

// EntryFactory describes a factory function that can generate structs for a specific versioned type
type EntryFactory func() EntryImpl

func NewProposedEntry(ctx context.Context, kind, version string, props ArtifactProperties) (models.ProposedEntry, error) {
	if tf, found := TypeMap.Load(kind); found {
		t := tf.(func() TypeImpl)()
		if t == nil {
			return nil, fmt.Errorf("error generating object for kind '%v'", kind)
		}
		return t.CreateProposedEntry(ctx, version, props)
	}
	return nil, fmt.Errorf("could not create entry for kind '%v'", kind)
}

// CreateVersionedEntry returns the specific instance for the type and version specified in the doc
// This method should be used on the insertion flow, which validates that the specific version proposed
// is permitted to be entered into the log.
func CreateVersionedEntry(pe models.ProposedEntry) (EntryImpl, error) {
	ei, err := UnmarshalEntry(pe)
	if err != nil {
		return nil, err
	}
	kind := pe.Kind()
	if tf, found := TypeMap.Load(kind); found {
		if !tf.(func() TypeImpl)().IsSupportedVersion(ei.APIVersion()) {
			return nil, fmt.Errorf("entry kind '%v' does not support inserting entries of version '%v'", kind, ei.APIVersion())
		}
	} else {
		return nil, fmt.Errorf("unknown kind '%v' specified", kind)
	}

	if ok, err := ei.Insertable(); !ok {
		return nil, fmt.Errorf("entry not insertable into log: %w", err)
	}

	return ei, nil
}

// UnmarshalEntry returns the specific instance for the type and version specified in the doc
// This method does not check for whether the version of the entry could be currently inserted into the log,
// and is useful when dealing with entries that have been persisted to the log.
func UnmarshalEntry(pe models.ProposedEntry) (EntryImpl, error) {
	if pe == nil {
		return nil, errors.New("proposed entry cannot be nil")
	}

	kind := pe.Kind()
	if tf, found := TypeMap.Load(kind); found {
		t := tf.(func() TypeImpl)()
		if t == nil {
			return nil, fmt.Errorf("error generating object for kind '%v'", kind)
		}
		return t.UnmarshalEntry(pe)
	}
	return nil, fmt.Errorf("could not unmarshal entry for kind '%v'", kind)
}

// DecodeEntry maps the (abstract) input structure into the specific entry implementation class;
// while doing so, it detects the case where we need to convert from string to []byte and does
// the base64 decoding required to make that happen.
// This also detects converting from string to strfmt.DateTime
func DecodeEntry(input, output interface{}) error {
	cfg := mapstructure.DecoderConfig{
		DecodeHook: func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
			if f.Kind() != reflect.String || t.Kind() != reflect.Slice && t != reflect.TypeOf(strfmt.DateTime{}) {
				return data, nil
			}

			if data == nil {
				return nil, errors.New("attempted to decode nil data")
			}

			if t == reflect.TypeOf(strfmt.DateTime{}) {
				return strfmt.ParseDateTime(data.(string))
			}

			bytes, err := base64.StdEncoding.DecodeString(data.(string))
			if err != nil {
				return []byte{}, fmt.Errorf("failed parsing base64 data: %w", err)
			}
			return bytes, nil
		},
		Result: output,
	}

	dec, err := mapstructure.NewDecoder(&cfg)
	if err != nil {
		return fmt.Errorf("error initializing decoder: %w", err)
	}

	return dec.Decode(input)
}

// CanonicalizeEntry returns the entry marshalled in JSON according to the
// canonicalization rules of RFC8785 to protect against any changes in golang's JSON
// marshalling logic that may reorder elements
func CanonicalizeEntry(ctx context.Context, entry EntryImpl) ([]byte, error) {
	canonicalEntry, err := entry.Canonicalize(ctx)
	if err != nil {
		return nil, err
	}
	return jsoncanonicalizer.Transform(canonicalEntry)
}

// ArtifactProperties provide a consistent struct for passing values from
// CLI flags to the type+version specific CreateProposeEntry() methods
type ArtifactProperties struct {
	AdditionalAuthenticatedData []byte
	ArtifactPath                *url.URL
	ArtifactHash                string
	ArtifactBytes               []byte
	SignaturePath               *url.URL
	SignatureBytes              []byte
	PublicKeyPaths              []*url.URL
	PublicKeyBytes              [][]byte
	PKIFormat                   string
}
