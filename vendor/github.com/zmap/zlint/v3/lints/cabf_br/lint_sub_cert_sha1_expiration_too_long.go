package cabf_br

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
	"time"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type sha1ExpireLong struct{}

/***************************************************************************************************************
Effective 16 January 2015, CAs SHOULD NOT issue Subscriber Certificates utilizing the SHA‐1 algorithm with
an Expiry Date greater than 1 January 2017 because Application Software Providers are in the process of
deprecating and/or removing the SHA‐1 algorithm from their software, and they have communicated that
CAs and Subscribers using such certificates do so at their own risk.
****************************************************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_sub_cert_sha1_expiration_too_long",
		Description:   "Subscriber certificates using the SHA-1 algorithm SHOULD NOT have an expiration date later than 1 Jan 2017",
		Citation:      "BRs: 7.1.3",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: time.Date(2015, time.January, 16, 0, 0, 0, 0, time.UTC),
		Lint:          &sha1ExpireLong{},
	})
}

func (l *sha1ExpireLong) Initialize() error {
	return nil
}

func (l *sha1ExpireLong) CheckApplies(c *x509.Certificate) bool {
	return !util.IsCACert(c) && (c.SignatureAlgorithm == x509.SHA1WithRSA ||
		c.SignatureAlgorithm == x509.DSAWithSHA1 ||
		c.SignatureAlgorithm == x509.ECDSAWithSHA1)
}

func (l *sha1ExpireLong) Execute(c *x509.Certificate) *lint.LintResult {
	if c.NotAfter.After(time.Date(2017, time.January, 1, 0, 0, 0, 0, time.UTC)) {
		return &lint.LintResult{Status: lint.Warn}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
