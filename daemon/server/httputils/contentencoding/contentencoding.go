// Package contentencoding provides utilities for negotiating HTTP content
// encodings.
package contentencoding

import (
	"net/http"

	"github.com/golang/gddo/httputil"
)

// Negotiate returns the best offered content encoding for the request's
// Accept-Encoding header. If multiple offers match with equal weight, the
// offer that appears earlier in the list is preferred.
//
// It returns "identity" if no content encoding is selected and an unencoded
// response is acceptable, or an empty string if none of the offered encodings
// and identity are acceptable.
func Negotiate(requestHeaders http.Header, offers []string) string {
	req := &http.Request{Header: requestHeaders}
	return httputil.NegotiateContentEncoding(req, offers)
}
