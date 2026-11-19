package v4

import (
	"net/url"
	"sort"
	"strings"
)

const doubleSpace = "  "

// StripExcessSpaces will rewrite the passed in slice's string values to not
// contain multiple side-by-side spaces.
func StripExcessSpaces(str string) string {
	var j, k, l, m, spaces int
	// Trim trailing spaces
	for j = len(str) - 1; j >= 0 && str[j] == ' '; j-- {
	}

	// Trim leading spaces
	for k = 0; k < j && str[k] == ' '; k++ {
	}
	str = str[k : j+1]

	// Strip multiple spaces.
	j = strings.Index(str, doubleSpace)
	if j < 0 {
		return str
	}

	buf := []byte(str)
	for k, m, l = j, j, len(buf); k < l; k++ {
		if buf[k] == ' ' {
			if spaces == 0 {
				// First space.
				buf[m] = buf[k]
				m++
			}
			spaces++
		} else {
			// End of multiple spaces.
			spaces = 0
			buf[m] = buf[k]
			m++
		}
	}

	return string(buf[:m])
}

// GetURIPath returns the escaped URI component from the provided URL.
func GetURIPath(u *url.URL) string {
	var uriPath string

	if len(u.Opaque) > 0 {
		const schemeSep, pathSep, queryStart = "//", "/", "?"

		opaque := u.Opaque
		// Cut off the query string if present.
		if idx := strings.Index(opaque, queryStart); idx >= 0 {
			opaque = opaque[:idx]
		}

		// Cutout the scheme separator if present.
		if strings.Index(opaque, schemeSep) == 0 {
			opaque = opaque[len(schemeSep):]
		}

		// capture URI path starting with first path separator.
		if idx := strings.Index(opaque, pathSep); idx >= 0 {
			uriPath = opaque[idx:]
		}
	} else {
		uriPath = u.EscapedPath()
	}

	if len(uriPath) == 0 {
		uriPath = "/"
	}

	return uriPath
}

// EncodeQuery returns the canonical query string for the given parameters, as
// required by the SigV4 canonical request.
//
// Each parameter name and value is URI-encoded individually, and the encoded
// name/value pairs are then sorted. The SigV4 specification is explicit that
// "the sorting occurs after encoding":
//
//	https://docs.aws.amazon.com/IAM/latest/UserGuide/create-signed-request.html
//
// The order of encoded and decoded forms can disagree, so the two are not
// interchangeable. The values "a/b" and "a-b" encode to "a%2Fb" and "a-b":
// '/' (0x2F) sorts after '-' (0x2D), but '%' (0x25) sorts before it, so the
// pair is ordered one way before encoding and the other way after. Sorting
// before encoding therefore produces a canonical request the service does not
// agree with, and the request is rejected as unauthorized.
//
// Unlike url.Values.Encode, a space is encoded as "%20" rather than "+", which
// is what SigV4 requires.
func EncodeQuery(query url.Values) string {
	type queryParam struct {
		key   string
		value string
	}

	params := make([]queryParam, 0, len(query))
	for key, values := range query {
		encodedKey := encodeQueryComponent(key)
		for _, value := range values {
			params = append(params, queryParam{
				key:   encodedKey,
				value: encodeQueryComponent(value),
			})
		}
	}

	sort.Slice(params, func(i, j int) bool {
		if params[i].key != params[j].key {
			return params[i].key < params[j].key
		}
		return params[i].value < params[j].value
	})

	var canonical strings.Builder
	for i, param := range params {
		if i > 0 {
			canonical.WriteByte('&')
		}
		canonical.WriteString(param.key)
		canonical.WriteByte('=')
		canonical.WriteString(param.value)
	}

	return canonical.String()
}

// encodeQueryComponent URI-encodes a single query string name or value the way
// SigV4 requires: every character outside the RFC 3986 unreserved set
// (A-Z a-z 0-9 - _ . ~) is percent-encoded with uppercase hex, and a space
// becomes "%20".
//
// url.QueryEscape already matches this, except that it encodes a space as "+".
// A literal '+' is escaped to "%2B", so replacing '+' only rewrites spaces.
func encodeQueryComponent(s string) string {
	return strings.Replace(url.QueryEscape(s), "+", "%20", -1)
}
