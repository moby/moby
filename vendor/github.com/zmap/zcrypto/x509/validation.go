// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import "time"

// Validation stores different validation levels for a given certificate
type Validation struct {
	BrowserTrusted bool   `json:"browser_trusted"`
	BrowserError   string `json:"browser_error,omitempty"`
	MatchesDomain  bool   `json:"matches_domain,omitempty"`
	Domain         string `json:"-"`
}

// ValidateWithStupidDetail fills out a Validation struct given a leaf
// certificate and intermediates / roots. If opts.DNSName is set, then it will
// also check if the domain matches.
//
// Deprecated: Use verifier.Verify() instead.
func (c *Certificate) ValidateWithStupidDetail(opts VerifyOptions) (chains []CertificateChain, validation *Validation, err error) {

	// Manually set the time, so that all verifies we do get the same time
	if opts.CurrentTime.IsZero() {
		opts.CurrentTime = time.Now()
	}

	// XXX: Don't pass a KeyUsage to the Verify API
	opts.KeyUsages = nil
	domain := opts.DNSName
	opts.DNSName = ""

	out := new(Validation)
	out.Domain = domain

	if chains, _, _, err = c.Verify(opts); err != nil {
		out.BrowserError = err.Error()
	} else {
		out.BrowserTrusted = true
	}

	if domain != "" {
		nameErr := c.VerifyHostname(domain)
		if nameErr != nil {
			out.MatchesDomain = false
		} else {
			out.MatchesDomain = true
		}

		// Make sure we return an error if either chain building or hostname
		// verification fails.
		if err == nil && nameErr != nil {
			err = nameErr
		}
	}
	validation = out

	return
}
