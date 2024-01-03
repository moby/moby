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
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

var (
	// Any of the following x509.SignatureAlgorithms are acceptable per §6.1.5 of
	// the BRs.
	passSigAlgs = map[x509.SignatureAlgorithm]bool{
		x509.SHA256WithRSA:   true,
		x509.SHA384WithRSA:   true,
		x509.SHA512WithRSA:   true,
		x509.DSAWithSHA256:   true,
		x509.ECDSAWithSHA256: true,
		x509.ECDSAWithSHA384: true,
		x509.ECDSAWithSHA512: true,
		// NOTE: BRs section §6.1.5 does not include SHA1 digest algorithms in the
		// current version. We allow these here for historic reasons and check for
		// SHA1 usage after the deprecation date in the separate
		// `e_sub_cert_or_sub_ca_using_sha1` lint.
		x509.SHA1WithRSA:   true,
		x509.DSAWithSHA1:   true,
		x509.ECDSAWithSHA1: true,
	}
	// The BRs do not forbid the use of RSA-PSS as a signature scheme in
	// certificates but it is not broadly supported by user-agents. Since
	// the BRs do not forbid the practice we return a warning result.
	// NOTE: The Mozilla root program policy *does* forbid their use since v2.7.
	// This should be covered by a lint scoped to the Mozilla source instead of in
	// this CABF lint.
	warnSigAlgs = map[x509.SignatureAlgorithm]bool{
		x509.SHA256WithRSAPSS: true,
		x509.SHA384WithRSAPSS: true,
		x509.SHA512WithRSAPSS: true,
	}
)

type signatureAlgorithmNotSupported struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_signature_algorithm_not_supported",
		Description:   "Certificates MUST meet the following requirements for algorithm Source: SHA-1*, SHA-256, SHA-384, SHA-512",
		Citation:      "BRs: 6.1.5",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.ZeroDate,
		Lint:          &signatureAlgorithmNotSupported{},
	})
}

func (l *signatureAlgorithmNotSupported) Initialize() error {
	return nil
}

func (l *signatureAlgorithmNotSupported) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *signatureAlgorithmNotSupported) Execute(c *x509.Certificate) *lint.LintResult {
	sigAlg := c.SignatureAlgorithm
	status := lint.Error
	if passSigAlgs[sigAlg] {
		status = lint.Pass
	} else if warnSigAlgs[sigAlg] {
		status = lint.Warn
	}
	return &lint.LintResult{
		Status: status,
	}
}
