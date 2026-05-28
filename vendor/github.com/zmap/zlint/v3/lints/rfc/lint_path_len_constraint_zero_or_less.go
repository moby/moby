package rfc

/*
 * ZLint Copyright 2023 Regents of the University of Michigan
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
	"github.com/zmap/zcrypto/encoding/asn1"
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type basicConst struct {
	CA                bool `asn1:"optional"`
	PathLenConstraint int  `asn1:"optional"`
}

type pathLenNonPositive struct {
}

/********************************************************************
The pathLenConstraint field is meaningful only if the cA boolean is
asserted and the key usage extension, if present, asserts the
keyCertSign bit (Section 4.2.1.3).  In this case, it gives the
maximum number of non-self-issued intermediate certificates that may
follow this certificate in a valid certification path.  (Note: The
last certificate in the certification path is not an intermediate
certificate, and is not included in this limit.  Usually, the last
certificate is an end entity certificate, but it can be a CA
certificate.)  A pathLenConstraint of zero indicates that no non-
self-issued intermediate CA certificates may follow in a valid
certification path.  Where it appears, the pathLenConstraint field
MUST be greater than or equal to zero.  Where pathLenConstraint does
not appear, no limit is imposed.
********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_path_len_constraint_zero_or_less",
		Description:   "Where it appears, the pathLenConstraint field MUST be greater than or equal to zero",
		Citation:      "RFC 5280: 4.2.1.9",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewPathLenNonPositive,
	})
}

func NewPathLenNonPositive() lint.LintInterface {
	return &pathLenNonPositive{}
}

func (l *pathLenNonPositive) CheckApplies(cert *x509.Certificate) bool {
	return cert.BasicConstraintsValid
}

func (l *pathLenNonPositive) Execute(cert *x509.Certificate) *lint.LintResult {
	var bc basicConst

	ext := util.GetExtFromCert(cert, util.BasicConstOID)
	if _, err := asn1.Unmarshal(ext.Value, &bc); err != nil {
		return &lint.LintResult{Status: lint.Fatal}
	}
	if bc.PathLenConstraint < 0 {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
