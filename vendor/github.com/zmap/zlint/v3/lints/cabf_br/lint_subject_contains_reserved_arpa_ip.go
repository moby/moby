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

const (
	// arpaTLD holds a string constant for the .arpa TLD
	arpaTLD = ".arpa"

	// rdnsIPv4Suffix is the expected suffix for IPv4 reverse DNS names as
	// specified in https://tools.ietf.org/html/rfc1035#section-3.5
	rdnsIPv4Suffix = ".in-addr" + arpaTLD
	// rndsIPv4Labels is the expected number of labels for an IPv4 reverse DNS
	// name (not counting the rdnsIPv4Suffix labels). IPv4 addresses are four
	// bytes. RFC 1035 uses one byte per label meaning there are 4 expected labels
	// under the rdnsIPv4Suffix.
	rdnsIPv4Labels = 4

	// rdnsIPv6Suffix is the expected suffix for IPv6 reverse DNS names as
	// specified in https://tools.ietf.org/html/rfc3596#section-2.5
	rdnsIPv6Suffix = ".ip6" + arpaTLD
	// rndsIPv6Labels is the expected number of labels for an IPv6 reverse DNS
	// name (not counting the rdnsIPv6Suffix labels). IPv6 addresses are 16 bytes.
	// RFC 3596 Sec 2.5 uses one *nibble* per label meaning there are 16*2
	// expected labels under the rdnsIPv6Suffix.
	rdnsIPv6Labels = 32
)

// arpaReservedIP is a linter that errors for any well formed rDNS names in the
// .in-addr.arpa or .ip6.arpa zones that specify an address in an IANA reserved
// network.
// See also: lint_subject_contains_malformed_arpa_ip.go for a lint that warns
// about malformed rDNS names in these zones.
type arpaReservedIP struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_contains_reserved_arpa_ip",
		Description:   "Checks no subject domain name contains a rDNS entry in an .arpa zone specifying a reserved IP address",
		Citation:      "BRs: 7.1.4.2.1",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewArpaReservedIP,
	})
}

func NewArpaReservedIP() lint.LintInterface {
	return &arpaReservedIP{}
}

// Initialize for an arpaReservedIP linter is a NOP to statisfy linting
// interfaces.

// CheckApplies returns true if the certificate contains any names that end in
// one of the two designated zones for reverse DNS: in-addr.arpa or ip6.arpa.
func (l *arpaReservedIP) CheckApplies(c *x509.Certificate) bool {
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
// subject alternate names that specify a well formed reverse DNS name under the
// respective IPv4 or IPv6 arpa zones do not specify an IP in an IANA
// reserved IP space. An lint.Error lint.LintResult is returned if the name specifies an
// IP address of the wrong class, or specifies an IP address in an IANA reserved
// network.
func (l *arpaReservedIP) Execute(c *x509.Certificate) *lint.LintResult {
	for _, name := range c.DNSNames {
		name = strings.ToLower(name)
		var err error
		if strings.HasSuffix(name, rdnsIPv4Suffix) {
			// If the name has the in-addr.arpa suffix then it should be an IPv4 reverse
			// DNS name.
			err = lintReversedIPAddress(name, false)
		} else if strings.HasSuffix(name, rdnsIPv6Suffix) {
			// If the name has the ip6.arpa suffix then it should be an IPv6 reverse
			// DNS name.
			err = lintReversedIPAddress(name, true)
		}
		// Return the first error as a negative lint result
		if err != nil {
			return &lint.LintResult{
				Status:  lint.Error,
				Details: err.Error(),
			}
		}
	}

	return &lint.LintResult{
		Status: lint.Pass,
	}
}

// reversedLabelsToIPv4 reverses the provided labels (assumed to be 4 labels,
// one per byte of the IPv6 address) and constructs an IPv4 address, returning
// the result of calling net.ParseIP for the constructed address.
func reversedLabelsToIPv4(labels []string) net.IP {
	var buf strings.Builder

	// If there aren't the right number of labels, it isn't an IPv4 address.
	if len(labels) != rdnsIPv4Labels {
		return nil
	}

	// An IPv4 address is represented as four groups of bytes separated by '.'
	for i := len(labels) - 1; i >= 0; i-- {
		buf.WriteString(labels[i])
		if i != 0 {
			buf.WriteString(".")
		}
	}
	return net.ParseIP(buf.String())
}

// reversedLabelsToIPv6 reverses the provided labels (assumed to be 32 labels,
// one per nibble of an IPv6 address) and constructs an IPv6 address, returning
// the result of calling net.ParseIP for the constructed address.
func reversedLabelsToIPv6(labels []string) net.IP {
	var buf strings.Builder

	// If there aren't the right number of labels, it isn't an IPv6 address.
	if len(labels) != rdnsIPv6Labels {
		return nil
	}

	// An IPv6 address is represented as eight groups of two bytes separated
	// by `:` in hex form. Since each label in the rDNS form is one nibble we need
	// four label components per IPv6 address component group.
	for i := len(labels) - 1; i >= 0; i -= 4 {
		buf.WriteString(labels[i])
		buf.WriteString(labels[i-1])
		buf.WriteString(labels[i-2])
		buf.WriteString(labels[i-3])
		if i > 4 {
			buf.WriteString(":")
		}
	}
	return net.ParseIP(buf.String())
}

// lintReversedIPAddress lints the given name as either a reversed IPv4 or IPv6
// address under the respective ARPA zone based on the address class. An error
// is returned if:
//
//  1. The IP address labels parse as an IP of the wrong address class for the
//     arpa suffix the name is using.
//  2. The IP address is within an IANA reserved range.
func lintReversedIPAddress(name string, ipv6 bool) error {
	numRequiredLabels := rdnsIPv4Labels
	zoneSuffix := rdnsIPv4Suffix

	if ipv6 {
		numRequiredLabels = rdnsIPv6Labels
		zoneSuffix = rdnsIPv6Suffix
	}

	// Strip off the zone suffix to get only the reversed IP address
	ipName := strings.TrimSuffix(name, zoneSuffix)

	// A well encoded IPv4 or IPv6 reverse DNS name will have the correct number
	// of labels to express the address. If there isn't the right number of labels
	// a separate `lint_subject_contains_malformed_arpa_ip.go` linter will flag it
	// as a warning. This linter is specifically concerned with well formed rDNS
	// that specifies a reserved IP.
	ipLabels := strings.Split(ipName, ".")
	if len(ipLabels) != numRequiredLabels {
		return nil
	}

	// Reverse the IP labels and try to parse an IP address
	var ip net.IP
	if ipv6 {
		ip = reversedLabelsToIPv6(ipLabels)
	} else {
		ip = reversedLabelsToIPv4(ipLabels)
	}
	// If the result isn't an IP at all assume there is no problem - leave
	// `lint_subject_contains_malformed_arpa_ip` to flag it as a warning.
	if ip == nil {
		return nil
	}

	if !ipv6 && ip.To4() == nil {
		// If we weren't expecting IPv6 and got it, that's a problem
		return fmt.Errorf(
			"the first %d labels of name %q parsed as a reversed IPv6 address but is "+
				"in the %q IPv4 reverse DNS zone.",
			numRequiredLabels, name, rdnsIPv4Suffix)
	} else if ipv6 && ip.To4() != nil {
		// If we were expecting IPv6 and got an IPv4 address, that's a problem
		return fmt.Errorf(
			"the first %d labels of name %q parsed as a reversed IPv4 address but is "+
				"in the %q IPv4 reverse DNS zone.",
			numRequiredLabels, name, rdnsIPv6Suffix)
	}

	// If the IP address is in an IANA reserved space, that's a problem.
	if util.IsIANAReserved(ip) {
		return fmt.Errorf(
			"the first %d labels of name %q parsed as a reversed IP address in "+
				"an IANA reserved IP space.",
			numRequiredLabels, name)
	}

	return nil
}
