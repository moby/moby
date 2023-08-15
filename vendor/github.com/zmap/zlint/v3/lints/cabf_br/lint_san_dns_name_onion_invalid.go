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

package cabf_br

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

var (
	// Per 2.4 of Rendezvous v2:
	//   Valid onion addresses contain 16 characters in a-z2-7 plus ".onion"
	onionV2Len = 16

	// Per 1.2 of Rendezvous v3:
	//   A hidden service's name is its long term master identity key.  This is
	//   encoded as a hostname by encoding the entire key in Base 32, including
	//   a version byte and a checksum, and then appending the string ".onion"
	//   at the end. The result is a 56-character domain name.
	onionV3Len = 56

	// Per RFC 4648, Section 6, the Base-32 alphabet is A-Z, 2-7, and =.
	// Because v2/v3 addresses are always aligned, they should never be padded,
	// and so omit = from the character set, as it's also not permitted in a
	// domain in the "preferred name syntax". Because `.onion` names appear in
	// DNS, which is case insensitive, the alphabet is extended to include a-z,
	// as the names are tested for well-formedness prior to normalization to
	// uppercase.
	base32SubsetRegex = regexp.MustCompile(`^[a-zA-Z2-7]+$`)
)

type onionNotValid struct{}

/*******************************************************************
https://tools.ietf.org/html/rfc7686#section-1

   Note that .onion names are required to conform with DNS name syntax
   (as defined in Section 3.5 of [RFC1034] and Section 2.1 of
   [RFC1123]), as they will still be exposed to DNS implementations.

   See [tor-address] and [tor-rendezvous] for the details of the
   creation and use of .onion names.

Baseline Requirements, v1.6.9, Appendix C (Ballot SC27)

The Domain Name MUST contain at least two labels, where the right-most label
is "onion", and the label immediately preceding the right-most "onion" label
is a valid Version 3 Onion Address, as defined in section 6 of the Tor
Rendezvous Specification - Version 3 located at
https://spec.torproject.org/rend-spec-v3.

Explanation:
Since CA/Browser Forum Ballot 144, `.onion` names have been permitted,
predating the ratification of RFC 7686. RFC 7686 introduced a normative
dependency on the Tor address and rendezvous specifications, which describe
v2 addresses. As the EV Guidelines have, since v1.5.3, required that the CA
obtain a demonstration of control from the Applicant, which effectively
requires the `.onion` name to be well-formed, even prior to RFC 7686.

See also https://github.com/cabforum/documents/issues/191
*******************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_san_dns_name_onion_invalid",
		Description:   "certificates with a .onion subject name must be issued in accordance with the Tor address/rendezvous specification",
		Citation:      "RFC 7686, EVGs v1.7.2: Appendix F, BRs v1.6.9: Appendix C",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.OnionOnlyEVDate,
		Lint:          &onionNotValid{},
	})
}

func (l *onionNotValid) Initialize() error {
	return nil
}

// CheckApplies returns true if the certificate contains one or more subject
// names ending in `.onion`.
func (l *onionNotValid) CheckApplies(c *x509.Certificate) bool {
	// TODO(sleevi): This should also be extended to support nameConstraints
	// in the future.
	return util.CertificateSubjInTLD(c, util.OnionTLD)
}

// Execute will lint the provided certificate. A lint.Error lint.LintResult will
// be returned if:
//
//  1) The certificate contains a Tor Rendezvous Spec v2 address and is not an
//     EV certificate (BRs: Appendix C).
//  2) The certificate contains a `.onion` subject name/SAN that is neither a
//     Rendezvous Spec v2 or v3 address.
func (l *onionNotValid) Execute(c *x509.Certificate) *lint.LintResult {
	for _, subj := range append(c.DNSNames, c.Subject.CommonName) {
		if !strings.HasSuffix(subj, util.OnionTLD) {
			continue
		}
		labels := strings.Split(subj, ".")
		if len(labels) < 2 {
			return &lint.LintResult{
				Status: lint.Error,
				Details: fmt.Sprintf("certificate contained a %s domain with too "+
					"few labels: %q", util.OnionTLD, subj),
			}
		}
		onionDomain := labels[len(labels)-2]
		if len(onionDomain) == onionV2Len {
			// Onion v2 address. These are only permitted for EV, per BRs Appendix C.
			if !util.IsEV(c.PolicyIdentifiers) {
				return &lint.LintResult{
					Status: lint.Error,
					Details: fmt.Sprintf("%q is a v2 address, but the certificate is not "+
						"EV", subj),
				}
			}
		} else if len(onionDomain) == onionV3Len {
			// Onion v3 address. Permitted for all certificates by CA/Browser Forum
			// Ballot SC27.
		} else {
			return &lint.LintResult{
				Status:  lint.Error,
				Details: fmt.Sprintf("%q is not a v2 or v3 Tor address", subj),
			}
		}
		if !base32SubsetRegex.MatchString(onionDomain) {
			return &lint.LintResult{
				Status: lint.Error,
				Details: fmt.Sprintf("%q contains invalid characters not permitted "+
					"within base-32", subj),
			}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
