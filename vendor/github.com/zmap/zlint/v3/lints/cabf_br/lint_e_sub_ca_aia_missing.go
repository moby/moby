package cabf_br

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
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type caAiaMissing struct{}

/***********************************************
CAB 7.1.2.2c
With the exception of stapling, which is noted below, this extension MUST be present. It MUST NOT be
marked critical, and it MUST contain the HTTP URL of the Issuing CA’s OCSP responder (accessMethod
= 1.3.6.1.5.5.7.48.1). It SHOULD also contain the HTTP URL of the Issuing CA’s certificate
(accessMethod = 1.3.6.1.5.5.7.48.2).
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:            "e_sub_ca_aia_missing",
		Description:     "Subordinate CA Certificate: authorityInformationAccess MUST be present, with the exception of stapling.",
		Citation:        "BRs: 7.1.2.2",
		Source:          lint.CABFBaselineRequirements,
		EffectiveDate:   util.CABEffectiveDate,
		IneffectiveDate: util.CABFBRs_1_7_1_Date,
		Lint:            NewCaAiaMissing,
	})
}

func NewCaAiaMissing() lint.LintInterface {
	return &caAiaMissing{}
}

func (l *caAiaMissing) CheckApplies(c *x509.Certificate) bool {
	return util.IsCACert(c) && !util.IsRootCA(c)
}

func (l *caAiaMissing) Execute(c *x509.Certificate) *lint.LintResult {
	if util.IsExtInCert(c, util.AiaOID) {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error}
	}
}
