package contenttype

import (
	"net/http"

	"github.com/golang/gddo/httputil"
	"github.com/golang/gddo/httputil/header"
)

// MatchAcceptStrict returns the best matching offer explicitly listed in the
// request's Accept header, ignoring wildcard media ranges ("*/*" and "type/*").
//
// Only exact media-type matches are considered. Entries with q=0 are ignored.
// If multiple offers match, the one with the highest q-value is selected.
// If q-values tie, the offer that appears earlier in the offers slice wins.
//
// MatchAcceptStrict returns "" if the Accept header is not present or no
// explicit match is found.
func MatchAcceptStrict(requestHeaders http.Header, offers []string) string {
	accept := requestHeaders.Get("Accept")
	if accept == "" {
		return ""
	}

	specs := header.ParseAccept(requestHeaders, "Accept")

	best := ""
	bestQ := -1.0

	for _, offer := range offers {
		for _, spec := range specs {
			if spec.Q == 0 || spec.Value != offer {
				continue
			}
			if spec.Q > bestQ {
				bestQ = spec.Q
				best = offer
			}
		}
	}

	return best
}

// Negotiate returns the best offered content type for the request's
// Accept header. If two offers match with equal weight, then the more specific
// offer is preferred.  For example, text/* trumps */*. If two offers match
// with equal weight and specificity, then the offer earlier in the list is
// preferred. If no offers match, then defaultOffer is returned.
func Negotiate(requestHeaders http.Header, offers []string, defaultOffer string) string {
	req := &http.Request{Header: requestHeaders}
	return httputil.NegotiateContentType(req, offers, defaultOffer)
}
