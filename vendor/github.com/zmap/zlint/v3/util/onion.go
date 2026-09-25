package util

import (
	"encoding/base32"
	"strings"

	"github.com/zmap/zcrypto/x509"
)

// An onion address is base32 encoded, however Tor believes that the standard base32 encoding
// is lowercase while the Go standard library believes that the standard base32 encoding is uppercase.
//
// onionBase32Encoding is simply base32.StdEncoding but lowercase instead of uppercase in order
// to work with the above mismatch.
var onionBase32Encoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567")

// IsOnionV3Address returns whether or not the provided DNS name is an Onion V3 encoded address.
//
// In order to be an Onion V3 encoded address, the DNS name must satisfy the following:
//  1. Contain at least two labels.
//  2. The right most label MUST be "onion".
//  3. The second to the right most label MUST be exactly 56 characters long.
//  4. The second to the right most label MUST be base32 encoded against the lowercase standard encoding.
//  5. The final byte of the decoded result from #4 MUST be equal to 0x03.
func IsOnionV3Address(dnsName string) bool {
	labels := strings.Split(dnsName, ".")
	if len(labels) < 2 || labels[len(labels)-1] != "onion" {
		return false
	}
	address := labels[len(labels)-2]
	if len(address) != 56 {
		return false
	}
	raw, err := onionBase32Encoding.DecodeString(address)
	if err != nil {
		return false
	}
	return raw[len(raw)-1] == 0x03
}

// IsOnionV2Address returns whether-or-not the give address appears to be an Onion V2 address.
//
// In order to be an Onion V2 encoded address, the DNS name must satisfy the following:
//  1. The address has at least two labels.
//  2. The right most label is the .onion TLD.
//  3. The second-to-the-right most label is a 16 character long, base32.
func IsOnionV2Address(dnsName string) bool {
	if !strings.HasSuffix(dnsName, "onion") {
		return false
	}
	labels := strings.Split(dnsName, ".")
	if len(labels) < 2 {
		return false
	}
	if len(labels[0]) != 16 {
		return false
	}
	_, err := onionBase32Encoding.DecodeString(labels[0])
	return err == nil
}

// IsOnionV3Cert returns whether-or-not at least one of the provided certificates subject common name,
// or any of its DNS names, are version 3 Onion addresses.
func IsOnionV3Cert(c *x509.Certificate) bool {
	return anyAreOnionVX(append(c.DNSNames, c.Subject.CommonName), IsOnionV3Address)
}

// IsOnionV2Cert returns whether-or-not at least one of the provided certificates subject common name,
// or any of its DNS names, are version 2 Onion addresses.
func IsOnionV2Cert(c *x509.Certificate) bool {
	return anyAreOnionVX(append(c.DNSNames, c.Subject.CommonName), IsOnionV2Address)
}

// anyAreOnionVX returns whether-or-not there is at least one item
// within the given slice that satisfies the given predicate.
//
// An empty slice always returns `false`.
//
// @TODO once we commit to forcing the library users onto Go 1.18 this should migrate to a generic function.
func anyAreOnionVX(slice []string, predicate func(string) bool) bool {
	for _, item := range slice {
		if predicate(item) {
			return true
		}
	}
	return false
}

// allAreOnionVX returns whether-or-not all items within the given slice
// satisfy the given predicate.
//
// An empty slice always returns `true`. This may seem counterintuitive,
// however it is due to being what is called a "vacuous truth". For
// more information, please see https://en.wikipedia.org/wiki/Vacuous_truth.
//
// @TODO once we commit to forcing the library users onto Go 1.18 this should migrate to a generic function.
func allAreOnionVX(slice []string, predicate func(string) bool) bool {
	return !anyAreOnionVX(slice, func(item string) bool {
		return !predicate(item)
	})
}
