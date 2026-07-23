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
	"crypto/x509"
	"errors"
	"time"

	"github.com/sigstore/sigstore-go/pkg/root"
)

func VerifyLeafCertificate(observerTimestamp time.Time, leafCert *x509.Certificate, trustedMaterial root.TrustedMaterial) ([][]*x509.Certificate, error) { // nolint: revive
	return verifyLeafCertificate(observerTimestamp, leafCert, trustedMaterial, nil)
}

func verifyLeafCertificate(observerTimestamp time.Time, leafCert *x509.Certificate, trustedMaterial root.TrustedMaterial, intermediates []*x509.Certificate) ([][]*x509.Certificate, error) {
	for _, ca := range trustedMaterial.FulcioCertificateAuthorities() {
		if fca, ok := ca.(*root.FulcioCertificateAuthority); ok && len(intermediates) > 0 {
			withIntermediates := *fca
			withIntermediates.Intermediates = append([]*x509.Certificate{}, fca.Intermediates...)
			withIntermediates.Intermediates = append(withIntermediates.Intermediates, intermediates...)
			ca = &withIntermediates
		}
		chains, err := ca.Verify(leafCert, observerTimestamp)
		if err == nil {
			return chains, nil
		}
	}

	return nil, errors.New("leaf certificate verification failed")
}
