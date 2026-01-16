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

package certificate

import (
	"crypto/x509"
	"errors"
	"fmt"
	"reflect"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

type Summary struct {
	CertificateIssuer      string `json:"certificateIssuer"`
	SubjectAlternativeName string `json:"subjectAlternativeName"`
	Extensions
}

type ErrCompareExtensions struct {
	field    string
	expected string
	actual   string
}

func (e *ErrCompareExtensions) Error() string {
	return fmt.Sprintf("expected %s to be \"%s\", got \"%s\"", e.field, e.expected, e.actual)
}

func SummarizeCertificate(cert *x509.Certificate) (Summary, error) {
	extensions, err := ParseExtensions(cert.Extensions)

	if err != nil {
		return Summary{}, err
	}

	var san string

	switch {
	case len(cert.URIs) > 0:
		san = cert.URIs[0].String()
	case len(cert.EmailAddresses) > 0:
		san = cert.EmailAddresses[0]
	}
	if san == "" {
		san, _ = cryptoutils.UnmarshalOtherNameSAN(cert.Extensions)
	}
	if san == "" {
		return Summary{}, errors.New("no Subject Alternative Name found")
	}

	return Summary{CertificateIssuer: cert.Issuer.String(), SubjectAlternativeName: san, Extensions: extensions}, nil
}

// CompareExtensions compares two Extensions structs and returns an error if
// any set values in the expected struct not equal. Empty fields in the
// expectedExt struct are ignored.
func CompareExtensions(expectedExt, actualExt Extensions) error {
	expExtValue := reflect.ValueOf(expectedExt)
	actExtValue := reflect.ValueOf(actualExt)

	fields := reflect.VisibleFields(expExtValue.Type())
	for _, field := range fields {
		expectedFieldVal := expExtValue.FieldByName(field.Name)

		// if the expected field is empty, skip it
		if expectedFieldVal.IsValid() && !expectedFieldVal.IsZero() {
			actualFieldVal := actExtValue.FieldByName(field.Name)
			if actualFieldVal.IsValid() {
				if expectedFieldVal.Interface() != actualFieldVal.Interface() {
					return &ErrCompareExtensions{field.Name, fmt.Sprintf("%v", expectedFieldVal.Interface()), fmt.Sprintf("%v", actualFieldVal.Interface())}
				}
			}
		}
	}

	return nil
}
