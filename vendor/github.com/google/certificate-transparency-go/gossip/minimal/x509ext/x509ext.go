// Copyright 2018 Google LLC. All Rights Reserved.
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

// Package x509ext holds extensions types and values for minimal gossip.
package x509ext

import (
	"errors"
	"fmt"

	"github.com/google/certificate-transparency-go/asn1"
	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/certificate-transparency-go/x509"

	ct "github.com/google/certificate-transparency-go"
)

// OIDExtensionCTSTH is the OID value for an X.509 extension that holds
// a log STH value.
// TODO(drysdale): get an official OID value
var OIDExtensionCTSTH = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 4, 5}

// OIDExtKeyUsageCTMinimalGossip is the OID value for an extended key usage
// (EKU) that indicates a leaf certificate is used for the validation of STH
// values from public CT logs.
// TODO(drysdale): get an official OID value
var OIDExtKeyUsageCTMinimalGossip = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 4, 6}

// LogSTHInfo is the structure that gets TLS-encoded into the X.509 extension
// identified by OIDExtensionCTSTH.
type LogSTHInfo struct {
	LogURL            []byte   `tls:"maxlen:255"`
	Version           tls.Enum `tls:"maxval:255"`
	TreeSize          uint64
	Timestamp         uint64
	SHA256RootHash    ct.SHA256Hash
	TreeHeadSignature ct.DigitallySigned
}

// LogSTHInfoFromCert retrieves the STH information embedded in a certificate.
func LogSTHInfoFromCert(cert *x509.Certificate) (*LogSTHInfo, error) {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(OIDExtensionCTSTH) {
			var sthInfo LogSTHInfo
			rest, err := tls.Unmarshal(ext.Value, &sthInfo)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal STH: %v", err)
			} else if len(rest) > 0 {
				return nil, fmt.Errorf("trailing data (%d bytes) after STH", len(rest))
			}
			return &sthInfo, nil
		}
	}
	return nil, errors.New("no STH extension found")
}

// HasSTHInfo indicates whether a certificate has embedded STH information.
func HasSTHInfo(cert *x509.Certificate) bool {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(OIDExtensionCTSTH) {
			return true
		}
	}
	return false
}

// STHFromCert retrieves the STH embedded in a certificate; note the returned STH
// does not have the LogID field filled in.
func STHFromCert(cert *x509.Certificate) (*ct.SignedTreeHead, error) {
	sthInfo, err := LogSTHInfoFromCert(cert)
	if err != nil {
		return nil, err
	}
	return &ct.SignedTreeHead{
		Version:           ct.Version(sthInfo.Version),
		TreeSize:          sthInfo.TreeSize,
		Timestamp:         sthInfo.Timestamp,
		SHA256RootHash:    sthInfo.SHA256RootHash,
		TreeHeadSignature: sthInfo.TreeHeadSignature,
	}, nil
}
