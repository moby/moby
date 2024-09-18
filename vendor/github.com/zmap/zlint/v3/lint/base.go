package lint

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
	"github.com/zmap/zlint/v3/util"
)

// LintInterface is implemented by each Lint.
type LintInterface interface {
	// Initialize runs once per-lint. It is called during RegisterLint().
	Initialize() error

	// CheckApplies runs once per certificate. It returns true if the Lint should
	// run on the given certificate. If CheckApplies returns false, the Lint
	// result is automatically set to NA without calling CheckEffective() or
	// Run().
	CheckApplies(c *x509.Certificate) bool

	// Execute() is the body of the lint. It is called for every certificate for
	// which CheckApplies() returns true.
	Execute(c *x509.Certificate) *LintResult
}

// A Lint struct represents a single lint, e.g.
// "e_basic_constraints_not_critical". It contains an implementation of LintInterface.
type Lint struct {

	// Name is a lowercase underscore-separated string describing what a given
	// Lint checks. If Name beings with "w", the lint MUST NOT return Error, only
	// Warn. If Name beings with "e", the Lint MUST NOT return Warn, only Error.
	Name string `json:"name,omitempty"`

	// A human-readable description of what the Lint checks. Usually copied
	// directly from the CA/B Baseline Requirements or RFC 5280.
	Description string `json:"description,omitempty"`

	// The source of the check, e.g. "BRs: 6.1.6" or "RFC 5280: 4.1.2.6".
	Citation string `json:"citation,omitempty"`

	// Programmatic source of the check, BRs, RFC5280, or ZLint
	Source LintSource `json:"source"`

	// Lints automatically returns NE for all certificates where CheckApplies() is
	// true but with NotBefore < EffectiveDate. This check is bypassed if
	// EffectiveDate is zero.
	EffectiveDate time.Time `json:"-"`

	// The implementation of the lint logic.
	Lint LintInterface `json:"-"`
}

// CheckEffective returns true if c was issued on or after the EffectiveDate. If
// EffectiveDate is zero, CheckEffective always returns true.
func (l *Lint) CheckEffective(c *x509.Certificate) bool {
	if l.EffectiveDate.IsZero() || !l.EffectiveDate.After(c.NotBefore) {
		return true
	}
	return false
}

// Execute runs the lint against a certificate. For lints that are
// sourced from the CA/B Forum Baseline Requirements, we first determine
// if they are within the purview of the BRs. See LintInterface for details
// about the other methods called. The ordering is as follows:
//
// CheckApplies()
// CheckEffective()
// Execute()
func (l *Lint) Execute(cert *x509.Certificate) *LintResult {
	if l.Source == CABFBaselineRequirements && !util.IsServerAuthCert(cert) {
		return &LintResult{Status: NA}
	}
	if !l.Lint.CheckApplies(cert) {
		return &LintResult{Status: NA}
	} else if !l.CheckEffective(cert) {
		return &LintResult{Status: NE}
	}
	res := l.Lint.Execute(cert)
	return res
}
