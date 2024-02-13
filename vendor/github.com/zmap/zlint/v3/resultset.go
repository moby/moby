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

package zlint

import (
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
)

// ResultSet contains the output of running all lints in a registry against
// a single certificate.
type ResultSet struct {
	Version         int64                       `json:"version"`
	Timestamp       int64                       `json:"timestamp"`
	Results         map[string]*lint.LintResult `json:"lints"`
	NoticesPresent  bool                        `json:"notices_present"`
	WarningsPresent bool                        `json:"warnings_present"`
	ErrorsPresent   bool                        `json:"errors_present"`
	FatalsPresent   bool                        `json:"fatals_present"`
}

// Execute lints the given certificate with all of the lints in the provided
// registry. The ResultSet is mutated to trace the lint results obtained from
// linting the certificate.
func (z *ResultSet) execute(cert *x509.Certificate, registry lint.Registry) {
	z.Results = make(map[string]*lint.LintResult, len(registry.Names()))
	// Run each lints from the registry.
	for _, name := range registry.Names() {
		res := registry.ByName(name).Execute(cert)
		z.Results[name] = res
		z.updateErrorStatePresent(res)
	}
}

func (z *ResultSet) updateErrorStatePresent(result *lint.LintResult) {
	switch result.Status {
	case lint.Notice:
		z.NoticesPresent = true
	case lint.Warn:
		z.WarningsPresent = true
	case lint.Error:
		z.ErrorsPresent = true
	case lint.Fatal:
		z.FatalsPresent = true
	}
}
