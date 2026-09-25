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

// Package cryptoutils implements support for working with encoded certificates, public keys, and private keys
package cryptoutils

import (
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"time"
)

const (
	// CertificatePEMType is the string "CERTIFICATE" to be used during PEM encoding and decoding
	CertificatePEMType PEMType = "CERTIFICATE"
)

// MarshalCertificateToPEM converts the provided X509 certificate into PEM format
func MarshalCertificateToPEM(cert *x509.Certificate) ([]byte, error) {
	if cert == nil {
		return nil, errors.New("nil certificate provided")
	}
	return PEMEncode(CertificatePEMType, cert.Raw), nil
}

// MarshalCertificatesToPEM converts the provided X509 certificates into PEM format
func MarshalCertificatesToPEM(certs []*x509.Certificate) ([]byte, error) {
	buf := bytes.Buffer{}
	for _, cert := range certs {
		pemBytes, err := MarshalCertificateToPEM(cert)
		if err != nil {
			return nil, err
		}
		_, _ = buf.Write(pemBytes)
	}
	return buf.Bytes(), nil
}

// UnmarshalCertificatesFromPEM extracts one or more X509 certificates from the provided
// byte slice, which is assumed to be in PEM-encoded format.
func UnmarshalCertificatesFromPEM(pemBytes []byte) ([]*x509.Certificate, error) {
	result := []*x509.Certificate{}
	remaining := pemBytes
	remaining = bytes.TrimSpace(remaining)

	for len(remaining) > 0 {
		var certDer *pem.Block
		certDer, remaining = pem.Decode(remaining)

		if certDer == nil {
			return nil, errors.New("error during PEM decoding")
		}

		cert, err := x509.ParseCertificate(certDer.Bytes)
		if err != nil {
			return nil, err
		}
		result = append(result, cert)
	}
	return result, nil
}

// UnmarshalCertificatesFromPEMLimited extracts one or more X509 certificates from the provided
// byte slice, which is assumed to be in PEM-encoded format. Fails after a specified
// number of iterations. A reasonable limit is 10 iterations.
func UnmarshalCertificatesFromPEMLimited(pemBytes []byte, iterations int) ([]*x509.Certificate, error) {
	result := []*x509.Certificate{}
	remaining := pemBytes
	remaining = bytes.TrimSpace(remaining)

	count := 0
	for len(remaining) > 0 {
		if count == iterations {
			return nil, errors.New("too many certificates specified in PEM block")
		}
		var certDer *pem.Block
		certDer, remaining = pem.Decode(remaining)

		if certDer == nil {
			return nil, errors.New("error during PEM decoding")
		}

		cert, err := x509.ParseCertificate(certDer.Bytes)
		if err != nil {
			return nil, err
		}
		result = append(result, cert)
		count++
	}
	return result, nil
}

// LoadCertificatesFromPEM extracts one or more X509 certificates from the provided
// io.Reader.
func LoadCertificatesFromPEM(pem io.Reader) ([]*x509.Certificate, error) {
	fileBytes, err := io.ReadAll(pem)
	if err != nil {
		return nil, err
	}
	return UnmarshalCertificatesFromPEM(fileBytes)
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// CheckExpiration verifies that epoch is during the validity period of
// the certificate provided.
//
// It returns nil if issueTime < epoch < expirationTime, and error otherwise.
func CheckExpiration(cert *x509.Certificate, epoch time.Time) error {
	if cert == nil {
		return errors.New("certificate is nil")
	}
	if cert.NotAfter.Before(epoch) {
		return fmt.Errorf("certificate expiration time %s is before %s", formatTime(cert.NotAfter), formatTime(epoch))
	}
	if cert.NotBefore.After(epoch) {
		return fmt.Errorf("certificate issued time %s is before %s", formatTime(cert.NotBefore), formatTime(epoch))
	}
	return nil
}

// ParseCSR parses a PKCS#10 PEM-encoded CSR.
func ParseCSR(csr []byte) (*x509.CertificateRequest, error) {
	derBlock, _ := pem.Decode(csr)
	if derBlock == nil || derBlock.Bytes == nil {
		return nil, errors.New("no CSR found while decoding")
	}
	correctType := false
	acceptedHeaders := []string{"CERTIFICATE REQUEST", "NEW CERTIFICATE REQUEST"}
	for _, v := range acceptedHeaders {
		if derBlock.Type == v {
			correctType = true
		}
	}
	if !correctType {
		return nil, fmt.Errorf("DER type %v is not of any type %v for CSR", derBlock.Type, acceptedHeaders)
	}

	return x509.ParseCertificateRequest(derBlock.Bytes)
}

// GenerateSerialNumber creates a compliant serial number as per RFC 5280 4.1.2.2.
// Serial numbers must be positive, and can be no longer than 20 bytes.
// The serial number is generated with 159 bits, so that the first bit will always
// be 0, resulting in a positive serial number.
func GenerateSerialNumber() (*big.Int, error) {
	// Pick a random number from 0 to 2^159.
	serial, err := rand.Int(rand.Reader, (&big.Int{}).Exp(big.NewInt(2), big.NewInt(159), nil))
	if err != nil {
		return nil, errors.New("error generating serial number")
	}
	return serial, nil
}
