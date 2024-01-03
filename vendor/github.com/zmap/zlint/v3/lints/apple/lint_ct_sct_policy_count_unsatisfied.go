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

package apple

import (
	"fmt"
	"time"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zcrypto/x509/ct"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type sctPolicyCount struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_ct_sct_policy_count_unsatisfied",
		Description:   "Check if certificate has enough embedded SCTs to meet Apple CT Policy",
		Citation:      "https://support.apple.com/en-us/HT205280",
		Source:        lint.AppleRootStorePolicy,
		EffectiveDate: util.AppleCTPolicyDate,
		Lint:          &sctPolicyCount{},
	})
}

// Initialize for a sctPolicyCount instance does nothing.
func (l *sctPolicyCount) Initialize() error {
	return nil
}

// CheckApplies returns true for any subscriber certificates that are not
// precertificates (e.g. that do not have the CT poison extension defined in RFC
// 6962.
func (l *sctPolicyCount) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && !util.IsExtInCert(c, util.CtPoisonOID)
}

// Execute checks if the provided certificate has embedded SCTs from
// a sufficient number of unique CT logs to meet Apple's CT log policy[0],
// effective Oct 15th, 2018.
//
// The number of required SCTs from different logs is calculated based on the
// Certificate's lifetime. If the number of required SCTs are not embedded in
// the certificate a Notice level lint.LintResult is returned.
//
// | Certificate lifetime | # of SCTs from separate logs |
// -------------------------------------------------------
// | Less than 15 months  | 2                            |
// | 15 to 27 months      | 3                            |
// | 27 to 39 months      | 4                            |
// | More than 39 months  | 5                            |
// -------------------------------------------------------
//
// Important note 1: We can't know whether additional SCTs were presented
// alongside the certificate via OCSP stapling. This linter assumes only
// embedded SCTs are used and ignores the portion of the Apple policy related to
// SCTs delivered via OCSP. This is one limitation that restricts the linter's
// findings to Notice level. See more background discussion in Issue 226[1].
//
// Important note 2: The linter doesn't maintain a list of Apple's trusted
// logs. The SCTs embedded in the certificate may not be from log's Apple
// actually trusts. Similarly the embedded SCT signatures are not validated
// in any way.
//
// [0]: https://support.apple.com/en-us/HT205280
// [1]: https://github.com/zmap/zlint/issues/226
func (l *sctPolicyCount) Execute(c *x509.Certificate) *lint.LintResult {
	// Determine the required number of SCTs from separate logs
	expected := appleCTPolicyExpectedSCTs(c)

	// If there are no SCTs then the job is easy. We can return a Notice
	// lint.LintResult immediately.
	if len(c.SignedCertificateTimestampList) == 0 && expected > 0 {
		return &lint.LintResult{
			Status: lint.Notice,
			Details: fmt.Sprintf(
				"Certificate had 0 embedded SCTs. Browser policy may require %d for this certificate.",
				expected),
		}
	}

	// Build a map from LogID to SCT so that we can count embedded SCTs by unique
	// log.
	sctsByLogID := make(map[ct.SHA256Hash]*ct.SignedCertificateTimestamp)
	for _, sct := range c.SignedCertificateTimestampList {
		sctsByLogID[sct.LogID] = sct
	}

	// If the number of embedded SCTs from separate logs meets expected return
	// a lint.Pass result.
	if len(sctsByLogID) >= expected {
		return &lint.LintResult{Status: lint.Pass}
	}

	// Otherwise return a Notice result - there weren't enough SCTs embedded in
	// the certificate. More must be provided by OCSP stapling if the certificate
	// is to meet Apple's CT policy.
	return &lint.LintResult{
		Status: lint.Notice,
		Details: fmt.Sprintf(
			"Certificate had %d embedded SCTs from distinct log IDs. "+
				"Browser policy may require %d for this certificate.",
			len(sctsByLogID), expected),
	}
}

// appleCTPolicyExpectedSCTs returns a count of the number of SCTs expected to
// be embedded in the given certificate based on its lifetime.
//
// For this function the relevant portion of Apple's policy is the table
// "Number of embedded SCTs based on certificate lifetime" (Also reproduced in
// the `Execute` godoc comment).
func appleCTPolicyExpectedSCTs(cert *x509.Certificate) int {
	// Lifetime is relative to the certificate's NotBefore date.
	start := cert.NotBefore

	// Thresholds is an ordered array of lifetime periods and their expected # of
	// SCTs. A lifetime period is defined by the cutoff date relative to the
	// start of the certificate's lifetime.
	thresholds := []struct {
		CutoffDate time.Time
		Expected   int
	}{
		// Start date ... 15 months
		{CutoffDate: start.AddDate(0, 15, 0), Expected: 2},
		// Start date ... 27 months
		{CutoffDate: start.AddDate(0, 27, 0), Expected: 3},
		// Start date ... 39 months
		{CutoffDate: start.AddDate(0, 39, 0), Expected: 4},
	}

	// If the certificate's lifetime falls into any of the cutoff date ranges then
	// we expect that range's expected # of SCTs for this certificate. This loop
	// assumes the `thresholds` list is sorted in ascending order.
	for _, threshold := range thresholds {
		if cert.NotAfter.Before(threshold.CutoffDate) {
			return threshold.Expected
		}
	}

	// The certificate had a validity > 39 months.
	return 5
}
