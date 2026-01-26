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

package rfc

import (
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type nameConstraintNotFQDN struct{}

/***********************************************************************
   For URIs, the constraint applies to the host part of the name.  The
   constraint MUST be specified as a fully qualified domain name and MAY
   specify a host or a domain.  Examples would be "host.example.com" and
   ".example.com".  When the constraint begins with a period, it MAY be
   expanded with one or more labels.  That is, the constraint
   ".example.com" is satisfied by both host.example.com and
   my.host.example.com.  However, the constraint ".example.com" is not
   satisfied by "example.com".  When the constraint does not begin with
   a period, it specifies a host.
************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_name_constraint_not_fqdn",
		Description:   "For URIs, the constraint MUST be specified as a fully qualified domain name [...] When the constraint begins with a period, it MAY be expanded with one or more labels.",
		Citation:      "RFC 5280: 4.2.1.10",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC5280Date,
		Lint:          NewNameConstraintNotFQDN,
	})
}

func NewNameConstraintNotFQDN() lint.LintInterface {
	return &nameConstraintNotFQDN{}
}

func (l *nameConstraintNotFQDN) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.NameConstOID)
}

func (l *nameConstraintNotFQDN) Execute(c *x509.Certificate) *lint.LintResult {

	var incorrectPermittedHosts []string
	var incorrectExcludedHosts []string
	var errString string

	incorrectPermittedHosts = collectNotFQDNEntries(c.PermittedURIs)
	incorrectExcludedHosts = collectNotFQDNEntries(c.ExcludedURIs)

	if len(incorrectPermittedHosts) != 0 {
		errString += buildErrorString(incorrectPermittedHosts, true)
	}
	if len(incorrectPermittedHosts) != 0 && len(incorrectExcludedHosts) != 0 {
		errString += "; "
	}
	if len(incorrectExcludedHosts) != 0 {
		errString += buildErrorString(incorrectExcludedHosts, false)
	}

	if len(errString) != 0 {
		return &lint.LintResult{
			Status:  lint.Error,
			Details: errString,
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}

func collectNotFQDNEntries(hosts []x509.GeneralSubtreeString) []string {
	var incorrectHosts []string

	for _, subtreeString := range hosts {
		host := subtreeString.Data

		host = strings.TrimPrefix(host, ".")

		if !util.IsFQDN(host) {
			incorrectHosts = append(incorrectHosts, host)
		}
	}

	return incorrectHosts
}

func buildErrorString(incorrectHosts []string, isInclusion bool) string {

	errString := "certificate contained "

	if len(incorrectHosts) > 1 {
		errString += "multiple "
	} else {
		errString += "an "
	}

	if isInclusion {
		errString += "inclusion "
	} else {
		errString += "exclusion "
	}

	if len(incorrectHosts) > 1 {

		errString += "name constraints that are not fully qualified domain names: " + incorrectHosts[0]
		for _, incorrectHost := range incorrectHosts[1:] {
			util.AppendToStringSemicolonDelim(&errString, incorrectHost)
		}
		return errString

	}

	errString += "name constraint that is not a fully qualified domain name: " + incorrectHosts[0]
	return errString

}
