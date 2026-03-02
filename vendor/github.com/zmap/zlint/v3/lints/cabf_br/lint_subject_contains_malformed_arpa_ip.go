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
	"net"
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

// arpaMalformedIP is a linter that warns for malformed names under the
// .in-addr.arpa or .ip6.arpa zones.
// See also: lint_subject_contains_reserved_arpa_ip.go for a lint that ensures
// well formed rDNS names in these zones do not specify an address in a IANA
// reserved network.
type arpaMalformedIP struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name: "w_subject_contains_malformed_arpa_ip",
		Description: "Checks no subject domain name contains a rDNS entry in the " +
			"registry-controlled .arpa zone with the wrong number of labels, or " +
			"an invalid IP address (RFC 3596, BCP49)",
		// NOTE(@cpu): 3.2.2.6 is particular to wildcard domain validation for names
		// in a registry controlled zone (like .arpa), which would be an appropriate
		// citation for when this lint finds a rDNS entry with the wrong
		// number of labels/invalid IP because of the presence of a wildcard
		// character. There is a larger on-going discussion[0] on the BRs stance on
		// the .arpa zone entries that may produce a better citation to use here.
		//
		// [0]: https://github.com/cabforum/documents/issues/153
		Citation:      "BRs: 3.2.2.6",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewArpaMalformedIP,
	})
}

func NewArpaMalformedIP() lint.LintInterface {
	return &arpaMalformedIP{}
}

// Initialize for an arpaMalformedIP linter is a NOP to statisfy linting
// interfaces.

// CheckApplies returns true if the certificate contains any names that end in
// one of the two designated zones for reverse DNS: in-addr.arpa or ip6.arpa.
func (l *arpaMalformedIP) CheckApplies(c *x509.Certificate) bool {
	names := append([]string{c.Subject.CommonName}, c.DNSNames...)
	for _, name := range names {
		name = strings.ToLower(name)
		if strings.HasSuffix(name, rdnsIPv4Suffix) ||
			strings.HasSuffix(name, rdnsIPv6Suffix) {
			return true
		}
	}
	return false
}

// Execute will check the given certificate to ensure that all of the DNS
// subject alternate names that specify a reverse DNS name under the respective
// IPv4 or IPv6 arpa zones are well formed. A lint.Warn lint.LintResult is returned if
// the name is in a reverse DNS zone but has the wrong number of labels.
func (l *arpaMalformedIP) Execute(c *x509.Certificate) *lint.LintResult {
	for _, name := range c.DNSNames {
		name = strings.ToLower(name)
		var err error
		if strings.HasSuffix(name, rdnsIPv4Suffix) {
			// If the name has the in-addr.arpa suffix then it should be an IPv4 reverse
			// DNS name.
			err = lintReversedIPAddressLabels(name, false)
		} else if strings.HasSuffix(name, rdnsIPv6Suffix) {
			// If the name has the ip6.arpa suffix then it should be an IPv6 reverse
			// DNS name.
			err = lintReversedIPAddressLabels(name, true)
		}
		// Return the first error as a negative lint result
		if err != nil {
			return &lint.LintResult{
				Status:  lint.Warn,
				Details: err.Error(),
			}
		}
	}

	return &lint.LintResult{
		Status: lint.Pass,
	}
}

// lintReversedIPAddressLabels lints the given name as either a reversed IPv4 or
// IPv6 address under the respective ARPA zone based on the address class. An
// error is returned if there aren't enough labels in the name after removing
// the relevant arpa suffix.
func lintReversedIPAddressLabels(name string, ipv6 bool) error {
	numRequiredLabels := rdnsIPv4Labels
	zoneSuffix := rdnsIPv4Suffix

	if ipv6 {
		numRequiredLabels = rdnsIPv6Labels
		zoneSuffix = rdnsIPv6Suffix
	}

	// Strip off the zone suffix to get only the reversed IP address
	ipName := strings.TrimSuffix(name, zoneSuffix)

	// A well encoded IPv4 or IPv6 reverse DNS name will have the correct number
	// of labels to express the address
	ipLabels := strings.Split(ipName, ".")
	if len(ipLabels) != numRequiredLabels {
		return fmt.Errorf(
			"name %q has too few leading labels (%d vs %d) to be a reverse DNS entry "+
				"in the %q zone.",
			name, len(ipLabels), numRequiredLabels, zoneSuffix)
	}

	// Reverse the IP labels and try to parse an IP address
	var ip net.IP
	if ipv6 {
		ip = reversedLabelsToIPv6(ipLabels)
	} else {
		ip = reversedLabelsToIPv4(ipLabels)
	}

	// If the result isn't an IP then a warning should be generated
	if ip == nil {
		return fmt.Errorf(
			"the first %d labels of name %q did not parse as a reversed IP address",
			numRequiredLabels, name)
	}

	// Otherwise return no error - checking the actual value of the IP is left to
	// `lint_subject_contains_reserved_arpa_ip.go`.
	return nil
}
