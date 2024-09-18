package community

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
	"fmt"
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type pubSuffix struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "n_san_iana_pub_suffix_empty",
		Description:   "The domain SHOULD NOT have a bare public suffix",
		Citation:      "awslabs certlint",
		Source:        lint.Community,
		EffectiveDate: util.ZeroDate,
		Lint:          &pubSuffix{},
	})
}

func (l *pubSuffix) Initialize() error {
	return nil
}

func (l *pubSuffix) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.SubjectAlternateNameOID)
}

func (l *pubSuffix) Execute(c *x509.Certificate) *lint.LintResult {
	var badNames []string
	for _, parsedName := range c.GetParsedDNSNames(false) {
		if parseErr := parsedName.ParseError; parseErr == nil {
			continue
		} else if strings.HasSuffix(parseErr.Error(), "is a suffix") {
			badNames = append(badNames, parsedName.DomainString)
		}
	}

	if badNamesCount := len(badNames); badNamesCount > 0 {
		return &lint.LintResult{
			Status: lint.Notice,
			Details: fmt.Sprintf(
				"%d DNS name(s) are bare public suffixes: %s",
				badNamesCount, strings.Join(badNames, ", ")),
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}
