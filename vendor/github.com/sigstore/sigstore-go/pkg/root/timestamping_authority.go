// Copyright 2024 The Sigstore Authors.
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

package root

import (
	"bytes"
	"crypto/x509"
	"errors"
	"time"

	tsaverification "github.com/sigstore/timestamp-authority/v2/pkg/verification"
)

type Timestamp struct {
	Time time.Time
	URI  string
}

type TimestampingAuthority interface {
	Verify(signedTimestamp []byte, signatureBytes []byte) (*Timestamp, error)
}

type SigstoreTimestampingAuthority struct {
	Root                *x509.Certificate
	Intermediates       []*x509.Certificate
	Leaf                *x509.Certificate
	ValidityPeriodStart time.Time
	ValidityPeriodEnd   time.Time
	URI                 string
}

var _ TimestampingAuthority = &SigstoreTimestampingAuthority{}

func (tsa *SigstoreTimestampingAuthority) Verify(signedTimestamp []byte, signatureBytes []byte) (*Timestamp, error) {
	if tsa.Root == nil {
		var tsaURIDisplay string
		if tsa.URI != "" {
			tsaURIDisplay = tsa.URI + " "
		}
		return nil, errors.New("timestamping authority " + tsaURIDisplay + "root certificate is nil")
	}
	trustedRootVerificationOptions := tsaverification.VerifyOpts{
		Roots:          []*x509.Certificate{tsa.Root},
		Intermediates:  tsa.Intermediates,
		TSACertificate: tsa.Leaf,
	}

	// Ensure timestamp responses are from trusted sources
	timestamp, err := tsaverification.VerifyTimestampResponse(signedTimestamp, bytes.NewReader(signatureBytes), trustedRootVerificationOptions)
	if err != nil {
		return nil, err
	}

	if !tsa.ValidityPeriodStart.IsZero() && timestamp.Time.Before(tsa.ValidityPeriodStart) {
		return nil, errors.New("timestamp is before the validity period start")
	}
	if !tsa.ValidityPeriodEnd.IsZero() && timestamp.Time.After(tsa.ValidityPeriodEnd) {
		return nil, errors.New("timestamp is after the validity period end")
	}

	// All above verification successful, so return nil
	return &Timestamp{Time: timestamp.Time, URI: tsa.URI}, nil
}
