package lint

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
	"time"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/util"
)

// LintInterface is implemented by each certificate linter.
//
// @deprecated - use CertificateLintInterface instead.
type LintInterface = CertificateLintInterface

// RevocationListLintInterface is implemented by each revocation list linter.
type RevocationListLintInterface interface {
	// CheckApplies runs once per revocation list. It returns true if the
	// Lint should run on the given certificate. If CheckApplies returns
	// false, the Lint result is automatically set to NA without calling
	// CheckEffective() or Run().
	CheckApplies(r *x509.RevocationList) bool

	// Execute is the body of the lint. It is called for every revocation list
	// for which CheckApplies returns true.
	Execute(r *x509.RevocationList) *LintResult
}

// CertificateLintInterface is implemented by each certificate linter.
type CertificateLintInterface interface {
	// CheckApplies runs once per certificate. It returns true if the Lint should
	// run on the given certificate. If CheckApplies returns false, the Lint
	// result is automatically set to NA without calling CheckEffective() or
	// Run().
	CheckApplies(c *x509.Certificate) bool

	// Execute is the body of the lint. It is called for every certificate for
	// which CheckApplies returns true.
	Execute(c *x509.Certificate) *LintResult
}

// Configurable lints return a pointer into a struct that they wish to receive their configuration into.
type Configurable interface {
	Configure() interface{}
}

// LintMetadata represents the metadata that are broadly associated across all types of lints.
//
// That is, all lints (irrespective of being a certificate lint, a CRL lint, and OCSP, etc.)
// have a Name, a Description, a Citation, and so on.
//
// In this way, this struct may be embedded in any linting type in order to maintain this
// data, while each individual linting type provides the behavior over this data.
type LintMetadata struct {
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
	// EffectiveDate is zero. Please see CheckEffective for more information.
	EffectiveDate time.Time `json:"-"`

	// Lints automatically returns NE for all certificates where CheckApplies() is
	// true but with NotBefore >= IneffectiveDate. This check is bypassed if
	// IneffectiveDate is zero. Please see CheckEffective for more information.
	IneffectiveDate time.Time `json:"-"`
}

// A Lint struct represents a single lint, e.g.
// "e_basic_constraints_not_critical". It contains an implementation of LintInterface.
//
// @deprecated - use CertificateLint instead.
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
	// EffectiveDate is zero. Please see CheckEffective for more information.
	EffectiveDate time.Time `json:"-"`

	// Lints automatically returns NE for all certificates where CheckApplies() is
	// true but with NotBefore >= IneffectiveDate. This check is bypassed if
	// IneffectiveDate is zero. Please see CheckEffective for more information.
	IneffectiveDate time.Time `json:"-"`
	// A constructor which returns the implementation of the lint logic.
	Lint func() LintInterface `json:"-"`
}

// toCertificateLint converts a Lint to a CertificateLint for backwards compatibility.
//
// @deprecated - Use CertificateLint directly.
func (l *Lint) toCertificateLint() *CertificateLint {
	return &CertificateLint{
		LintMetadata: LintMetadata{
			Name:            l.Name,
			Description:     l.Description,
			Citation:        l.Citation,
			Source:          l.Source,
			EffectiveDate:   l.EffectiveDate,
			IneffectiveDate: l.IneffectiveDate,
		},
		Lint: l.Lint,
	}
}

// CheckEffective returns true if c was issued on or after the EffectiveDate
// AND before (but not on) the Ineffective date. That is, CheckEffective
// returns true if...
//
//	c.NotBefore in [EffectiveDate, IneffectiveDate)
//
// If EffectiveDate is zero, then only IneffectiveDate is checked. Conversely,
// if IneffectiveDate is zero then only EffectiveDate is checked. If both EffectiveDate
// and IneffectiveDate are zero then CheckEffective always returns true.
//
// @deprecated - use CertificateLint instead.
func (l *Lint) CheckEffective(c *x509.Certificate) bool {
	return l.toCertificateLint().CheckEffective(c)
}

// Execute runs the lint against a certificate. For lints that are
// sourced from the CA/B Forum Baseline Requirements, we first determine
// if they are within the purview of the BRs. See LintInterface for details
// about the other methods called. The ordering is as follows:
//
// Configure() ----> only if the lint implements Configurable
// CheckApplies()
// CheckEffective()
// Execute()
//
// @deprecated - use CertificateLint instead
func (l *Lint) Execute(cert *x509.Certificate, config Configuration) *LintResult {
	return l.toCertificateLint().Execute(cert, config)
}

// CertificateLint represents a single x509 certificate linter.
type CertificateLint struct {
	// Metadata associated with the linter.
	LintMetadata
	// A constructor which returns the implementation of the linter.
	Lint func() CertificateLintInterface `json:"-"`
}

// toLint converts a CertificateLint to Lint for backwards compatibility
//
// @deprecated - use CertificateLint directly.
func (l *CertificateLint) toLint() *Lint {
	return &Lint{
		Name:            l.Name,
		Description:     l.Description,
		Citation:        l.Citation,
		Source:          l.Source,
		EffectiveDate:   l.EffectiveDate,
		IneffectiveDate: l.IneffectiveDate,
		Lint:            l.Lint,
	}
}

// CheckEffective returns true if c was issued on or after the EffectiveDate
// AND before (but not on) the Ineffective date. That is, CheckEffective
// returns true if...
//
//	c.NotBefore in [EffectiveDate, IneffectiveDate)
//
// If EffectiveDate is zero, then only IneffectiveDate is checked. Conversely,
// if IneffectiveDate is zero then only EffectiveDate is checked. If both EffectiveDate
// and IneffectiveDate are zero then CheckEffective always returns true.
func (l *CertificateLint) CheckEffective(c *x509.Certificate) bool {
	return checkEffective(l.EffectiveDate, l.IneffectiveDate, c.NotBefore)
}

// Execute runs the lint against a certificate. For lints that are
// sourced from the CA/B Forum Baseline Requirements, we first determine
// if they are within the purview of the BRs. See CertificateLintInterface
// for details about the other methods called.
// The ordering is as follows:
//
// Configure() ----> only if the lint implements Configurable
// CheckApplies()
// CheckEffective()
// Execute()
func (l *CertificateLint) Execute(cert *x509.Certificate, config Configuration) *LintResult {
	if l.Source == CABFBaselineRequirements && !util.IsServerAuthCert(cert) {
		return &LintResult{Status: NA}
	}
	lint := l.Lint()
	err := config.MaybeConfigure(lint, l.Name)
	if err != nil {
		return &LintResult{
			Status:  Fatal,
			Details: err.Error()}
	}
	if !lint.CheckApplies(cert) {
		return &LintResult{Status: NA}
	} else if !l.CheckEffective(cert) {
		return &LintResult{Status: NE}
	}
	return lint.Execute(cert)
}

// RevocationListLint represents a single x509 CRL linter.
type RevocationListLint struct {
	// Metadata associated with the linter.
	LintMetadata
	// A constructor which returns the implementation of the linter.
	Lint func() RevocationListLintInterface `json:"-"`
}

// CheckEffective returns true if r was generated on or after the EffectiveDate
// AND before (but not on) the Ineffective date. That is, CheckEffective
// returns true if...
//
//	r.ThisUpdate in [EffectiveDate, IneffectiveDate)
//
// If EffectiveDate is zero, then only IneffectiveDate is checked. Conversely,
// if IneffectiveDate is zero then only EffectiveDate is checked. If both EffectiveDate
// and IneffectiveDate are zero then CheckEffective always returns true.
func (l *RevocationListLint) CheckEffective(r *x509.RevocationList) bool {
	return checkEffective(l.EffectiveDate, l.IneffectiveDate, r.ThisUpdate)
}

// Execute runs the lint against a revocation list.
// The ordering is as follows:
//
// Configure() ----> only if the lint implements Configurable
// CheckApplies()
// CheckEffective()
// Execute()
func (l *RevocationListLint) Execute(r *x509.RevocationList, config Configuration) *LintResult {
	lint := l.Lint()
	err := config.MaybeConfigure(lint, l.Name)
	if err != nil {
		return &LintResult{
			Status:  Fatal,
			Details: err.Error()}
	}
	if !lint.CheckApplies(r) {
		return &LintResult{Status: NA}
	} else if !l.CheckEffective(r) {
		return &LintResult{Status: NE}
	}
	return lint.Execute(r)
}

// checkEffective returns true if target was generated on or after the EffectiveDate
// AND before (but not on) the Ineffective date. That is, CheckEffective
// returns true if...
//
//	target in [effective, ineffective)
//
// If effective is zero, then only ineffective is checked. Conversely,
// if ineffective is zero then only effect is checked. If both effective
// and ineffective are zero then checkEffective always returns true.
func checkEffective(effective, ineffective, target time.Time) bool {
	onOrAfterEffective := effective.IsZero() || util.OnOrAfter(target, effective)
	strictlyBeforeIneffective := ineffective.IsZero() || target.Before(ineffective)
	return onOrAfterEffective && strictlyBeforeIneffective
}
