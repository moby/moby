package rfc

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
	"net/url"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type uriRelative struct{}

/*************************************************************************
When the issuerAltName extension contains a URI, the name MUST be
stored in the uniformResourceIdentifier (an IA5String).  The name
MUST NOT be a relative URI, and it MUST follow the URI syntax and
encoding rules specified in [RFC3986].  The name MUST include both a
scheme (e.g., "http" or "ftp") and a scheme-specific-part.  URIs that
include an authority ([RFC3986], Section 3.2) MUST include a fully
qualified domain name or IP address as the host.  Rules for encoding
Internationalized Resource Identifiers (IRIs) are specified in
Section 7.4.
*************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_ian_uri_relative",
		Description:   "When issuerAltName extension is present and the URI is used, the name MUST NOT be a relative URI",
		Citation:      "RFC 5280: 4.2.1.7",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC5280Date,
		Lint:          NewUriRelative,
	})
}

func NewUriRelative() lint.LintInterface {
	return &uriRelative{}
}

func (l *uriRelative) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.IssuerAlternateNameOID)
}

func (l *uriRelative) Execute(c *x509.Certificate) *lint.LintResult {
	for _, uri := range c.IANURIs {
		parsed_uri, err := url.Parse(uri)

		if err != nil {
			return &lint.LintResult{Status: lint.Error}
		}

		if !parsed_uri.IsAbs() {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
