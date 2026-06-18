// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package negotiate

import (
	"net/http"
	"strings"

	"github.com/go-openapi/runtime/server-middleware/mediatype"
	"github.com/go-openapi/runtime/server-middleware/negotiate/header"
)

// Option configures [ContentType] behaviour.
type Option func(*options)

type options struct {
	ignoreParameters bool
	matchSuffix      bool
}

func optionsWithDefaults(opts []Option) options {
	var o options
	for _, apply := range opts {
		apply(&o)
	}

	return o
}

// WithIgnoreParameters returns an [Option] that strips MIME-type
// parameters from both Accept entries and offers before matching, restoring
// the behaviour the runtime had before v0.30.
//
// New code should leave parameters honoured (the default). This option
// exists for applications that depend on the looser pre-v0.30 match —
// most often because their producers and Accept clients use mismatched
// charset or version params that they treat as informational.
//
// Example — per-call opt-out:
//
//	chosen := negotiate.ContentType(r, offers, "",
//	    negotiate.WithIgnoreParameters(true),
//	)
//
// Example — server-wide opt-out (via [middleware.Context]):
//
//	ctx := middleware.NewContext(spec, api, nil).SetIgnoreParameters(true)
func WithIgnoreParameters(ignore bool) Option {
	return func(o *options) {
		o.ignoreParameters = ignore
	}
}

// WithMatchSuffix returns an [Option] that extends content
// negotiation to tolerate RFC 6839 structured-syntax suffix media
// types. When enabled, an Accept entry of "application/json"
// matches an offer of "application/vnd.api+json" and vice versa,
// for the suffixes recognised by the runtime (+json, +xml, +yaml).
//
// Default: strict (false). Use only when interoperating with
// clients or servers that do not strictly abide by the spec — for
// example, servers returning application/problem+json error
// responses against operations that only declare application/json
// in produces.
//
// Suffix matches always lose to exact and alias matches when those
// are on offer; see [mediatype.MatchKind] for the tier ordering.
//
// Example — per-call opt-in:
//
//	chosen := negotiate.ContentType(r, offers, "",
//	    negotiate.WithMatchSuffix(true),
//	)
//
// Example — server-wide opt-in (via [middleware.Context]):
//
//	ctx := middleware.NewContext(spec, api, nil).SetMatchSuffix(true)
func WithMatchSuffix(enable bool) Option {
	return func(o *options) {
		o.matchSuffix = enable
	}
}

// ContentEncoding returns the best offered content encoding for the
// request's Accept-Encoding header. If two offers match with equal
// weight then the offer earlier in the list is preferred. If no offers
// are acceptable, then "" is returned.
//
// Encoding tokens have no parameters, so this function is unaffected by
// the v0.30 parameter-honouring change to [ContentType].
func ContentEncoding(r *http.Request, offers []string) string {
	bestOffer := "identity"
	bestQ := -1.0
	specs := header.ParseAccept(r.Header, "Accept-Encoding")
	for _, offer := range offers {
		for _, spec := range specs {
			if spec.Q > bestQ &&
				(spec.Value == "*" || spec.Value == offer) {
				bestQ = spec.Q
				bestOffer = offer
			}
		}
	}
	if bestQ == 0 {
		bestOffer = ""
	}

	return bestOffer
}

// ContentType returns the best offered content type for the request's
// Accept header. If two offers match with equal weight, then the more
// specific offer is preferred (text/* trumps */*; type/subtype trumps
// type/*). If two offers match with equal weight and specificity, then
// the offer earlier in the list is preferred. If no offers match, then
// defaultOffer is returned.
//
// As of v0.30 the matching rule honours MIME-type parameters: an Accept
// entry of "text/plain;charset=utf-8" matches an offer of bare
// "text/plain" (offer carries no constraint), but it does NOT match an
// offer of "text/plain;charset=ascii" (charset values disagree). Pass
// [WithIgnoreParameters](true) to restore the pre-v0.30 behaviour where
// parameters were stripped before matching — see [WithIgnoreParameters]
// for details and an example.
//
// When the Accept header is absent, the first offer is returned
// unchanged (param-stripping is irrelevant in that case).
func ContentType(r *http.Request, offers []string, defaultOffer string, opts ...Option) string {
	if len(offers) == 0 {
		return defaultOffer
	}
	o := optionsWithDefaults(opts)

	// Per RFC 7230 §3.2.2, multiple Accept headers are equivalent to a
	// single comma-joined value. Join before parsing so we don't drop
	// later entries.
	acceptValues := r.Header.Values("Accept")
	if len(acceptValues) == 0 {
		return offers[0]
	}
	acceptSet := mediatype.ParseAccept(strings.Join(acceptValues, ", "))
	if len(acceptSet) == 0 {
		return defaultOffer
	}

	offerSet := make(mediatype.Set, 0, len(offers))
	rawByIdx := make([]string, 0, len(offers))
	for _, raw := range offers {
		mt, err := mediatype.Parse(raw)
		if err != nil {
			continue
		}
		offerSet = append(offerSet, mt)
		rawByIdx = append(rawByIdx, raw)
	}
	if len(offerSet) == 0 {
		return defaultOffer
	}

	if o.ignoreParameters {
		acceptSet = stripSet(acceptSet)
		offerSet = stripSet(offerSet)
	}

	var matchOpts []mediatype.MatchOption
	if o.matchSuffix {
		matchOpts = append(matchOpts, mediatype.AllowSuffix())
	}
	best, ok := acceptSet.BestMatch(offerSet, matchOpts...)
	if !ok {
		return defaultOffer
	}
	// Return the original raw offer string so callers receive the value
	// they declared, with its parameters preserved.
	for i, mt := range offerSet {
		if mt.Type == best.Type && mt.Subtype == best.Subtype && sameParams(mt.Params, best.Params) {
			return rawByIdx[i]
		}
	}

	return best.String()
}

func stripSet(s mediatype.Set) mediatype.Set {
	out := make(mediatype.Set, len(s))
	for i, m := range s {
		out[i] = m.StripParams()
	}

	return out
}

func sameParams(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}

	return true
}
