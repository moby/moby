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

package util

import (
	"net"
	"net/url"
	"strings"

	zcutil "github.com/zmap/zcrypto/util"
	"github.com/zmap/zcrypto/x509"
)

func RemovePrependedQuestionMarks(domain string) string {
	for strings.HasPrefix(domain, "?.") {
		domain = domain[2:]
	}
	return domain
}

func RemovePrependedWildcard(domain string) string {
	return strings.TrimPrefix(domain, "*.")
}

func IsFQDN(domain string) bool {
	domain = RemovePrependedWildcard(domain)
	domain = RemovePrependedQuestionMarks(domain)
	return zcutil.IsURL(domain)
}

func GetAuthority(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	if parsed.Opaque != "" {
		// non-empty Opaque means that there is no authority
		return ""
	}
	if len(uri) < 4 {
		return ""
	}
	// https://tools.ietf.org/html/rfc3986#section-3
	// The only time an authority is present is if there is a // after the scheme.
	firstColon := strings.Index(uri, ":")
	postScheme := uri[firstColon+1:]
	// After the scheme, there is the hier-part, optionally followed by a query or fragment.
	if !strings.HasPrefix(postScheme, "//") {
		// authority is always prefixed by //
		return ""
	}
	for i := 2; i < len(postScheme); i++ {
		// in the hier-part, the authority is followed by either an absolute path, or the empty string.
		// So, the authority is terminated by the start of an absolute path (/), the start of a fragment (#) or the start of a query(?)
		if postScheme[i] == '/' || postScheme[i] == '#' || postScheme[i] == '?' {
			return postScheme[2:i]
		}
	}
	// Found no absolute path, fragment or query -- so the authority is the only data after the scheme://
	return postScheme[2:]
}

func GetHost(auth string) string {
	begin := strings.Index(auth, "@")
	if begin == len(auth)-1 {
		begin = -1
	}
	end := strings.Index(auth, ":")
	if end == -1 {
		end = len(auth)
	}
	if end < begin {
		return ""
	}
	return auth[begin+1 : end]
}

func AuthIsFQDNOrIP(auth string) bool {
	return IsFQDNOrIP(GetHost(auth))
}

func IsFQDNOrIP(host string) bool {
	if IsFQDN(host) {
		return true
	}
	if net.ParseIP(host) != nil {
		return true
	}
	return false
}

func DNSNamesExist(cert *x509.Certificate) bool {
	if cert.Subject.CommonName == "" && len(cert.DNSNames) == 0 {
		return false
	} else {
		return true
	}
}

func CommonNameIsIP(cert *x509.Certificate) bool {
	ip := net.ParseIP(cert.Subject.CommonName)
	if ip == nil {
		return false
	} else {
		return true
	}
}
