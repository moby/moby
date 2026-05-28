// Copyright 2022 The Sigstore Authors.
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

package cryptoutils

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
)

var (
	// OIDOtherName is the OID for the OtherName SAN per RFC 5280
	OIDOtherName = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 7}
	// SANOID is the OID for Subject Alternative Name per RFC 5280
	SANOID = asn1.ObjectIdentifier{2, 5, 29, 17}
)

// OtherName describes a name related to a certificate which is not in one
// of the standard name formats. RFC 5280, 4.2.1.6:
//
//	OtherName ::= SEQUENCE {
//	     type-id    OBJECT IDENTIFIER,
//	     value      [0] EXPLICIT ANY DEFINED BY type-id }
//
// OtherName for Fulcio-issued certificates only supports UTF-8 strings as values.
type OtherName struct {
	ID    asn1.ObjectIdentifier
	Value string `asn1:"utf8,explicit,tag:0"`
}

// MarshalOtherNameSAN creates a Subject Alternative Name extension
// with an OtherName sequence. RFC 5280, 4.2.1.6:
//
// SubjectAltName ::= GeneralNames
// GeneralNames ::= SEQUENCE SIZE (1..MAX) OF GeneralName
// GeneralName ::= CHOICE {
//
//	otherName                       [0]     OtherName,
//	... }
func MarshalOtherNameSAN(name string, critical bool) (*pkix.Extension, error) {
	o := OtherName{
		ID:    OIDOtherName,
		Value: name,
	}
	bytes, err := asn1.MarshalWithParams(o, "tag:0")
	if err != nil {
		return nil, err
	}

	sans, err := asn1.Marshal([]asn1.RawValue{{FullBytes: bytes}})
	if err != nil {
		return nil, err
	}
	return &pkix.Extension{
		Id:       SANOID,
		Critical: critical,
		Value:    sans,
	}, nil
}

// UnmarshalOtherNameSAN extracts a UTF-8 string from the OtherName
// field in the Subject Alternative Name extension.
func UnmarshalOtherNameSAN(exts []pkix.Extension) (string, error) {
	var otherNames []string

	for _, e := range exts {
		if !e.Id.Equal(SANOID) {
			continue
		}

		var seq asn1.RawValue
		rest, err := asn1.Unmarshal(e.Value, &seq)
		if err != nil {
			return "", err
		} else if len(rest) != 0 {
			return "", fmt.Errorf("trailing data after X.509 extension")
		}
		if !seq.IsCompound || seq.Tag != asn1.TagSequence || seq.Class != asn1.ClassUniversal {
			return "", asn1.StructuralError{Msg: "bad SAN sequence"}
		}

		rest = seq.Bytes
		for len(rest) > 0 {
			var v asn1.RawValue
			rest, err = asn1.Unmarshal(rest, &v)
			if err != nil {
				return "", err
			}

			// skip all GeneralName fields except OtherName
			if v.Tag != 0 {
				continue
			}

			var other OtherName
			if _, err := asn1.UnmarshalWithParams(v.FullBytes, &other, "tag:0"); err != nil {
				return "", fmt.Errorf("could not parse requested OtherName SAN: %w", err)
			}
			if !other.ID.Equal(OIDOtherName) {
				return "", fmt.Errorf("unexpected OID for OtherName, expected %v, got %v", OIDOtherName, other.ID)
			}
			otherNames = append(otherNames, other.Value)
		}
	}

	if len(otherNames) == 0 {
		return "", errors.New("no OtherName found")
	}
	if len(otherNames) != 1 {
		return "", errors.New("expected only one OtherName")
	}

	return otherNames[0], nil
}

// GetSubjectAlternateNames extracts all subject alternative names from
// the certificate, including email addresses, DNS, IP addresses, URIs,
// and OtherName SANs
func GetSubjectAlternateNames(cert *x509.Certificate) []string {
	sans := []string{}
	if cert == nil {
		return sans
	}
	sans = append(sans, cert.DNSNames...)
	sans = append(sans, cert.EmailAddresses...)
	for _, ip := range cert.IPAddresses {
		sans = append(sans, ip.String())
	}
	for _, uri := range cert.URIs {
		sans = append(sans, uri.String())
	}
	// ignore error if there's no OtherName SAN
	otherName, _ := UnmarshalOtherNameSAN(cert.Extensions)
	if len(otherName) > 0 {
		sans = append(sans, otherName)
	}
	return sans
}
