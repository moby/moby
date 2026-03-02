//
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

package verification

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"hash"
	"io"
	"math/big"

	"github.com/digitorus/pkcs7"
	"github.com/digitorus/timestamp"
	"github.com/pkg/errors"
)

var (
	// EKUOID is the Extended Key Usage OID, per RFC 5280
	EKUOID = asn1.ObjectIdentifier{2, 5, 29, 37}
)

// VerifyOpts contains verification options for a RFC3161 timestamp
type VerifyOpts struct {
	// OID verifies that the TSR's OID has an expected value. Optional, used when
	// an alternative OID was passed with a request to the TSA
	OID asn1.ObjectIdentifier
	// TSACertificate verifies that the TSR uses the TSACertificate as expected. Optional if the TSR contains the TSA certificate
	TSACertificate *x509.Certificate
	// Intermediates verifies the TSR's certificate. Optional, used for chain building
	Intermediates []*x509.Certificate
	// Roots is the set of trusted root certificates that verifies the TSR's certificate
	Roots []*x509.Certificate
	// Nonce verifies that the TSR contains the expected nonce. Optional, used when
	// an optional nonce was passed with a request to the TSA
	Nonce *big.Int
	// CommonName verifies that the TSR certificate subject's Common Name matches the expected value. Optional
	CommonName string
}

// Verify the TSR's certificate identifier matches a provided TSA certificate
func verifyESSCertID(tsaCert *x509.Certificate, opts VerifyOpts) error {
	if opts.TSACertificate == nil {
		return nil
	}

	if !bytes.Equal(opts.TSACertificate.RawIssuer, tsaCert.RawIssuer) {
		return fmt.Errorf("TSR cert issuer does not match provided TSA cert issuer")
	}

	if opts.TSACertificate.SerialNumber.Cmp(tsaCert.SerialNumber) != 0 {
		return fmt.Errorf("TSR cert serial number does not match provided TSA cert serial number")
	}

	return nil
}

// Verify the leaf certificate's subject Common Name matches a provided Common Name
func verifySubjectCommonName(cert *x509.Certificate, opts VerifyOpts) error {
	if opts.CommonName == "" {
		return nil
	}

	if cert.Subject.CommonName != opts.CommonName {
		return fmt.Errorf("the certificate's subject Common Name %s does not match the provided Common Name %s", cert.Subject.CommonName, opts.CommonName)
	}
	return nil
}

// If embedded in the TSR, verify the TSR's leaf certificate matches a provided TSA certificate
func verifyEmbeddedLeafCert(tsaCert *x509.Certificate, opts VerifyOpts) error {
	if opts.TSACertificate != nil && !opts.TSACertificate.Equal(tsaCert) {
		return fmt.Errorf("certificate embedded in the TSR does not match the provided TSA certificate")
	}
	return nil
}

// Verify the leaf's EKU is set to critical, per RFC 3161 2.3
func verifyLeafCertCriticalEKU(cert *x509.Certificate) error {
	var criticalEKU bool
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(EKUOID) {
			criticalEKU = ext.Critical
			break
		}
	}
	if !criticalEKU {
		return errors.New("certificate must set EKU to critical")
	}
	return nil
}

func verifyLeafCert(ts timestamp.Timestamp, opts VerifyOpts) error {
	if len(ts.Certificates) == 0 && opts.TSACertificate == nil {
		return fmt.Errorf("leaf certificate must be present the in TSR or as a verify option")
	}

	errMsg := "failed to verify TSA certificate"

	var leafCert *x509.Certificate
	if len(ts.Certificates) != 0 {
		for _, c := range ts.Certificates {
			if !c.IsCA {
				leafCert = c
				break
			}
		}
		if leafCert == nil {
			return fmt.Errorf("no leaf certificate found in chain")
		}

		err := verifyEmbeddedLeafCert(leafCert, opts)
		if err != nil {
			return fmt.Errorf("%s: %w", errMsg, err)
		}
	} else {
		leafCert = opts.TSACertificate
	}

	err := verifyLeafCertCriticalEKU(leafCert)
	if err != nil {
		return fmt.Errorf("%s: %w", errMsg, err)
	}

	err = verifyESSCertID(leafCert, opts)
	if err != nil {
		return fmt.Errorf("%s: %w", errMsg, err)
	}

	err = verifySubjectCommonName(leafCert, opts)
	if err != nil {
		return fmt.Errorf("%s: %w", errMsg, err)
	}

	// verifies that the leaf certificate and any intermediate certificates
	// have EKU set to only time stamping usage
	err = verifyLeafAndIntermediatesTimestampingEKU(leafCert, opts)
	if err != nil {
		return fmt.Errorf("failed to verify EKU on leaf certificate: %w", err)
	}

	return nil
}

func verifyLeafExtendedKeyUsage(cert *x509.Certificate) error {
	certEKULen := len(cert.ExtKeyUsage)
	if certEKULen != 1 {
		return fmt.Errorf("certificate has %d extended key usages, expected only one", certEKULen)
	}

	if cert.ExtKeyUsage[0] != x509.ExtKeyUsageTimeStamping {
		return fmt.Errorf("leaf certificate EKU is not set to TimeStamping as required")
	}
	return nil
}

func verifyIntermediateExtendedKeyUsage(cert *x509.Certificate) error {
	// If no EKU specified it means unrestricted usage
	if len(cert.ExtKeyUsage) == 0 {
		return nil
	}

	allowsTimestampingUse := false
	for _, eku := range cert.ExtKeyUsage {
		if eku == x509.ExtKeyUsageTimeStamping || eku == x509.ExtKeyUsageAny {
			allowsTimestampingUse = true
			break
		}
	}

	if !allowsTimestampingUse {
		return errors.New("intermediate certificate does not allow Timestamping usage")
	}

	return nil
}

// Verify the leaf and intermediate certificates (called "EKU chaining") all
// have the appropriate extended key usage set.
// Leaf certificates must have exactly one EKU set to Timestamping
// Intermediates can have no EKU (unrestricted) or multiple EKUs,
// which need to include Timestamping or UsageAny.
func verifyLeafAndIntermediatesTimestampingEKU(leafCert *x509.Certificate, opts VerifyOpts) error {
	err := verifyLeafExtendedKeyUsage(leafCert)
	if err != nil {
		return fmt.Errorf("failed to verify EKU on leaf certificate: %w", err)
	}

	for _, cert := range opts.Intermediates {
		err := verifyIntermediateExtendedKeyUsage(cert)
		if err != nil {
			return fmt.Errorf("failed to verify EKU on intermediate certificate: %w", err)
		}
	}
	return nil
}

// Verify the OID of the TSR matches an expected OID
func verifyOID(oid []int, opts VerifyOpts) error {
	if opts.OID == nil {
		return nil
	}
	responseOID := opts.OID
	if len(oid) != len(responseOID) {
		return fmt.Errorf("OID lengths do not match")
	}
	for i, v := range oid {
		if v != responseOID[i] {
			return fmt.Errorf("OID content does not match")
		}
	}
	return nil
}

// Verify the nonce - Mostly important for when the response is first returned
func verifyNonce(requestNonce *big.Int, opts VerifyOpts) error {
	if opts.Nonce == nil {
		return nil
	}
	if opts.Nonce.Cmp(requestNonce) != 0 {
		return fmt.Errorf("incoming nonce %d does not match TSR nonce %d", requestNonce, opts.Nonce)
	}
	return nil
}

// VerifyTimestampResponse the timestamp response using a timestamp certificate chain.
func VerifyTimestampResponse(tsrBytes []byte, artifact io.Reader, opts VerifyOpts) (*timestamp.Timestamp, error) {
	// Verify the status of the TSR does not contain an error
	// handled by the timestamp.ParseResponse function
	ts, err := timestamp.ParseResponse(tsrBytes)
	if err != nil {
		pe := timestamp.ParseError("")
		if errors.As(err, &pe) {
			return nil, fmt.Errorf("timestamp response is not valid: %w", err)
		}
		return nil, fmt.Errorf("error parsing response into Timestamp: %w", err)
	}

	// verify the timestamp response signature using the provided certificate pool
	if err = verifyTSRWithChain(ts, opts); err != nil {
		return nil, err
	}

	if err = verifyNonce(ts.Nonce, opts); err != nil {
		return nil, err
	}

	if err = verifyOID(ts.Policy, opts); err != nil {
		return nil, err
	}

	if err = verifyLeafCert(*ts, opts); err != nil {
		return nil, err
	}

	// verify the hash in the timestamp response matches the artifact hash
	if err = verifyHashedMessages(ts.HashAlgorithm.New(), ts.HashedMessage, artifact); err != nil {
		return nil, err
	}

	// if the parsed timestamp is verified, return the timestamp
	return ts, nil
}

func verifyTSRWithChain(ts *timestamp.Timestamp, opts VerifyOpts) error {
	p7Message, err := pkcs7.Parse(ts.RawToken)
	if err != nil {
		return fmt.Errorf("error parsing hashed message: %w", err)
	}

	if len(opts.Roots) == 0 {
		return fmt.Errorf("no root certificates provided for verifying the certificate chain")
	}
	rootCertPool := x509.NewCertPool()
	for _, cert := range opts.Roots {
		if cert != nil {
			rootCertPool.AddCert(cert)
		}
	}
	if rootCertPool.Equal(x509.NewCertPool()) {
		return fmt.Errorf("no valid root certificates provided for verifying the certificate chain")
	}
	intermediateCertPool := x509.NewCertPool()
	for _, cert := range opts.Intermediates {
		if cert != nil {
			intermediateCertPool.AddCert(cert)
		}
	}

	x509Opts := x509.VerifyOptions{
		Roots:         rootCertPool,
		Intermediates: intermediateCertPool,
	}

	// if the PCKS7 object does not have any certificates set in the
	// Certificates field, the VerifyWithChain method will because it will be
	// unable to find a leaf certificate associated with a signer. Since the
	// leaf certificate issuer and serial number information is already part of
	// the PKCS7 object, adding the leaf certificate to the Certificates field
	// will allow verification to pass
	if p7Message.Certificates == nil && opts.TSACertificate != nil {
		p7Message.Certificates = []*x509.Certificate{opts.TSACertificate}
	}

	err = p7Message.VerifyWithOpts(x509Opts)
	if err != nil {
		return fmt.Errorf("error while verifying with chain: %w", err)
	}

	return nil
}

// Verify that the TSR's hashed message matches the digest of the artifact to be timestamped
func verifyHashedMessages(hashAlg hash.Hash, hashedMessage []byte, artifactReader io.Reader) error {
	h := hashAlg
	if _, err := io.Copy(h, artifactReader); err != nil {
		return fmt.Errorf("failed to create hash %w", err)
	}
	localHashedMsg := h.Sum(nil)

	if !bytes.Equal(localHashedMsg, hashedMessage) {
		return fmt.Errorf("hashed messages don't match")
	}

	return nil
}
