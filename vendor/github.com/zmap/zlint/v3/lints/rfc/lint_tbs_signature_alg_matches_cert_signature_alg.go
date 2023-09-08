/*
 * ZLint Copyright 2021 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package rfc

import (
	"bytes"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
	"golang.org/x/crypto/cryptobyte"
	cryptobyte_asn1 "golang.org/x/crypto/cryptobyte/asn1"
)

type mismatchingSigAlg struct{}

/*******************************************************************
RFC 5280: 4.1.1.2
[the Certificate signatureAlgorithm] field MUST contain the same
algorithm identifier as the signature field in the sequence
tbsCertificate
********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_cert_sig_alg_not_match_tbs_sig_alg",
		Description:   "Certificate signature field must match TBSCertificate signature field",
		Citation:      "RFC 5280, Section 4.1.1.2",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC5280Date,
		Lint:          &mismatchingSigAlg{},
	})
}

func (l *mismatchingSigAlg) Initialize() error {
	return nil
}

func (l *mismatchingSigAlg) CheckApplies(_ *x509.Certificate) bool {
	return true
}

func (l *mismatchingSigAlg) Execute(c *x509.Certificate) *lint.LintResult {
	// parse out certificate signatureAlgorithm
	input := cryptobyte.String(c.Raw)
	var cert cryptobyte.String
	if !input.ReadASN1(&cert, cryptobyte_asn1.SEQUENCE) {
		return &lint.LintResult{Status: lint.Fatal, Details: "error reading certificate"}
	}
	var tbsCert cryptobyte.String
	if !cert.ReadASN1(&tbsCert, cryptobyte_asn1.SEQUENCE) {
		return &lint.LintResult{Status: lint.Fatal, Details: "error reading certificate.tbsCertificate"}
	}
	var certSigAlg cryptobyte.String
	if !cert.ReadASN1(&certSigAlg, cryptobyte_asn1.SEQUENCE) {
		return &lint.LintResult{Status: lint.Fatal, Details: "error reading certificate.signatureAlgorithm"}
	}

	// parse out tbsCertificate signature
	if !tbsCert.SkipOptionalASN1(cryptobyte_asn1.Tag(0).Constructed().ContextSpecific()) {
		return &lint.LintResult{Status: lint.Fatal, Details: "error reading tbsCertificate.version"}
	}
	if !tbsCert.SkipASN1(cryptobyte_asn1.INTEGER) {
		return &lint.LintResult{Status: lint.Fatal, Details: "error reading tbsCertificate.serialNumber"}
	}
	var tbsSigAlg cryptobyte.String
	if !tbsCert.ReadASN1(&tbsSigAlg, cryptobyte_asn1.SEQUENCE) {
		return &lint.LintResult{Status: lint.Fatal, Details: "error reading tbsCertificate.signature"}
	}

	if !bytes.Equal(certSigAlg, tbsSigAlg) {
		return &lint.LintResult{Status: lint.Error}
	}

	return &lint.LintResult{Status: lint.Pass}
}
