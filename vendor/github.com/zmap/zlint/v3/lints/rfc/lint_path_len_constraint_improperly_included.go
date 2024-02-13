package rfc

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

import (
	"encoding/asn1"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type pathLenIncluded struct{}

/******************************************************************
RFC 5280: 4.2.1.9
CAs MUST NOT include the pathLenConstraint field unless the cA
boolean is asserted and the key usage extension asserts the
keyCertSign bit.
******************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_path_len_constraint_improperly_included",
		Description:   "CAs MUST NOT include the pathLenConstraint field unless the CA boolean is asserted and the keyCertSign bit is set",
		Citation:      "RFC 5280: 4.2.1.9",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC3280Date,
		Lint:          &pathLenIncluded{},
	})
}

func (l *pathLenIncluded) Initialize() error {
	return nil
}

func (l *pathLenIncluded) CheckApplies(cert *x509.Certificate) bool {
	return util.IsExtInCert(cert, util.BasicConstOID)
}

func (l *pathLenIncluded) Execute(cert *x509.Certificate) *lint.LintResult {
	bc := util.GetExtFromCert(cert, util.BasicConstOID)
	var seq asn1.RawValue
	var isCa bool
	_, err := asn1.Unmarshal(bc.Value, &seq)
	if err != nil {
		return &lint.LintResult{Status: lint.Fatal}
	}
	if len(seq.Bytes) == 0 {
		return &lint.LintResult{Status: lint.Pass}
	}
	rest, err := asn1.UnmarshalWithParams(seq.Bytes, &isCa, "optional")
	if err != nil {
		return &lint.LintResult{Status: lint.Fatal}
	}
	keyUsageValue := util.IsExtInCert(cert, util.KeyUsageOID)
	if len(rest) > 0 && (!cert.IsCA || !keyUsageValue || (keyUsageValue && cert.KeyUsage&x509.KeyUsageCertSign == 0)) {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
