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

package cabf_br

import (
	"fmt"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type onionNotEV struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_san_dns_name_onion_not_ev_cert",
		Description:   "certificates with a .onion subject name must be issued in accordance with EV Guidelines",
		Citation:      "CABF Ballot 144",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.OnionOnlyEVDate,
		Lint:          NewOnionNotEV,
	})
}

func NewOnionNotEV() lint.LintInterface {
	return &onionNotEV{}
}

// This lint only applies for certificates issued before CA/Browser Forum
// Ballot SC27, which permitted .onion within non-EV certificates
func (l *onionNotEV) CheckApplies(c *x509.Certificate) bool {
	return c.NotBefore.Before(util.CABFBRs_1_6_9_Date) &&
		util.IsSubscriberCert(c) &&
		util.CertificateSubjInTLD(c, util.OnionTLD)
}

// Execute returns an lint.Error lint.LintResult if the certificate is not an EV
// certificate. CheckApplies has already verified the certificate contains one
// or more `.onion` subjects and so it must be an EV certificate.
func (l *onionNotEV) Execute(c *x509.Certificate) *lint.LintResult {
	/*
	 * Effective May 1, 2015, each CA SHALL revoke all unexpired Certificates with an
	 * Internal Name using onion as the right-most label in an entry in the
	 * subjectAltName Extension or commonName field unless such Certificate was
	 * issued in accordance with Appendix F of the EV Guidelines.
	 */
	if !util.IsEV(c.PolicyIdentifiers) {
		return &lint.LintResult{
			Status: lint.Error,
			Details: fmt.Sprintf(
				"certificate contains one or more %s subject domains but is not an EV certificate",
				util.OnionTLD),
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
