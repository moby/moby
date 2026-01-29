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
	"errors"
	"fmt"

	"github.com/sigstore/sigstore-go/pkg/root"
)

const maxAllowedTimestamps = 32

// VerifySignedTimestamp verifies that the given entity has been timestamped
// by a trusted timestamp authority and that the timestamp is valid.
func VerifySignedTimestamp(entity SignedEntity, trustedMaterial root.TrustedMaterial) ([]*root.Timestamp, []error, error) { //nolint:revive
	signedTimestamps, err := entity.Timestamps()
	if err != nil {
		return nil, nil, err
	}

	// limit the number of timestamps to prevent DoS
	if len(signedTimestamps) > maxAllowedTimestamps {
		return nil, nil, fmt.Errorf("too many signed timestamps: %d > %d", len(signedTimestamps), maxAllowedTimestamps)
	}
	sigContent, err := entity.SignatureContent()
	if err != nil {
		return nil, nil, err
	}

	signatureBytes := sigContent.Signature()

	verifiedTimestamps := []*root.Timestamp{}
	var verificationErrors []error
	for _, timestamp := range signedTimestamps {
		verifiedSignedTimestamp, err := verifySignedTimestamp(timestamp, signatureBytes, trustedMaterial)
		if err != nil {
			verificationErrors = append(verificationErrors, err)
			continue
		}
		if isDuplicateTSA(verifiedTimestamps, verifiedSignedTimestamp) {
			verificationErrors = append(verificationErrors, fmt.Errorf("duplicate timestamps from the same authority, ignoring %s", verifiedSignedTimestamp.URI))
			continue
		}

		verifiedTimestamps = append(verifiedTimestamps, verifiedSignedTimestamp)
	}

	return verifiedTimestamps, verificationErrors, nil
}

// isDuplicateTSA checks if the given verified signed timestamp is a duplicate
// of any of the verified timestamps.
// This is used to prevent replay attacks and ensure a single compromised TSA
// cannot meet the threshold.
func isDuplicateTSA(verifiedTimestamps []*root.Timestamp, verifiedSignedTimestamp *root.Timestamp) bool {
	for _, ts := range verifiedTimestamps {
		if ts.URI == verifiedSignedTimestamp.URI {
			return true
		}
	}
	return false
}

// VerifySignedTimestamp verifies that the given entity has been timestamped
// by a trusted timestamp authority and that the timestamp is valid.
//
// The threshold parameter is the number of unique timestamps that must be
// verified.
func VerifySignedTimestampWithThreshold(entity SignedEntity, trustedMaterial root.TrustedMaterial, threshold int) ([]*root.Timestamp, error) { //nolint:revive
	verifiedTimestamps, verificationErrors, err := VerifySignedTimestamp(entity, trustedMaterial)
	if err != nil {
		return nil, err
	}
	if len(verifiedTimestamps) < threshold {
		return nil, fmt.Errorf("threshold not met for verified signed timestamps: %d < %d; error: %w", len(verifiedTimestamps), threshold, errors.Join(verificationErrors...))
	}
	return verifiedTimestamps, nil
}

func verifySignedTimestamp(signedTimestamp []byte, signatureBytes []byte, trustedMaterial root.TrustedMaterial) (*root.Timestamp, error) {
	timestampAuthorities := trustedMaterial.TimestampingAuthorities()

	var errs []error

	// Iterate through TSA certificate authorities to find one that verifies
	for _, tsa := range timestampAuthorities {
		ts, err := tsa.Verify(signedTimestamp, signatureBytes)
		if err == nil {
			return ts, nil
		}
		errs = append(errs, err)
	}

	return nil, fmt.Errorf("unable to verify signed timestamps: %w", errors.Join(errs...))
}

// TODO: remove below deprecated functions before 2.0

// Deprecated: use VerifySignedTimestamp instead.
func VerifyTimestampAuthority(entity SignedEntity, trustedMaterial root.TrustedMaterial) ([]*root.Timestamp, []error, error) { //nolint:revive
	return VerifySignedTimestamp(entity, trustedMaterial)
}

// Deprecated: use VerifySignedTimestampWithThreshold instead.
func VerifyTimestampAuthorityWithThreshold(entity SignedEntity, trustedMaterial root.TrustedMaterial, threshold int) ([]*root.Timestamp, error) { //nolint:revive
	return VerifySignedTimestampWithThreshold(entity, trustedMaterial, threshold)
}
