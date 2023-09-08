package rfc

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

type subDirAttrCrit struct{}

/************************************************
RFC 5280: 4.2.1.8
The subject directory attributes extension is used to convey
   identification attributes (e.g., nationality) of the subject.  The
   extension is defined as a sequence of one or more attributes.
   Conforming CAs MUST mark this extension as non-critical.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_subject_directory_attr_critical",
		Description:   "Conforming CAs MUST mark the Subject Directory Attributes extension as not critical",
		Citation:      "RFC 5280: 4.2.1.8",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          &subDirAttrCrit{},
	})
}

func (l *subDirAttrCrit) Initialize() error {
	return nil
}

func (l *subDirAttrCrit) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.SubjectDirAttrOID)
}

func (l *subDirAttrCrit) Execute(c *x509.Certificate) *lint.LintResult {
	if e := util.GetExtFromCert(c, util.SubjectDirAttrOID); e.Critical {
		return &lint.LintResult{Status: lint.Error}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
