// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mediatype

import (
	"strings"
)

// Set is a list of media types — typically the parsed value of an Accept
// header, or a list of server-side offers.
type Set []MediaType

// ParseAccept parses a comma-separated list of media types, as found in
// the Accept, Accept-Charset (etc.) HTTP headers. Malformed entries are
// skipped silently — be liberal in what you accept.
//
// An empty input returns nil.
func ParseAccept(s string) Set {
	parts := splitTopLevel(s)
	if len(parts) == 0 {
		return nil
	}
	out := make(Set, 0, len(parts))
	for _, p := range parts {
		mt, err := Parse(p)
		if err != nil {
			continue
		}
		out = append(out, mt)
	}

	return out
}

// BestMatch picks the offer most acceptable to the receiver's Accept
// entries. Selection follows RFC 7231 §5.3.2 plus tier-aware
// ranking:
//
//   - highest q-value wins;
//   - ties on q broken by the highest [MediaType.Specificity] of the
//     matching Accept entry;
//   - ties on specificity broken by [MatchKind] (MatchExact beats
//     MatchAlias beats MatchSuffix);
//   - ties on match kind broken by earliest position in offered.
//
// Accept entries with q=0 are treated as exclusions and never match.
// MatchSuffix results are only counted when [AllowSuffix] is passed.
// Returns ok=false if no offer matched any non-zero-q entry.
func (s Set) BestMatch(offered Set, opts ...MatchOption) (best MediaType, ok bool) {
	if len(s) == 0 || len(offered) == 0 {
		return MediaType{}, false
	}
	o := applyMatchOptions(opts)
	bestQ := -1.0
	bestSpec := -1
	bestKind := MatchNone
	bestIdx := -1
	for i, offer := range offered {
		for _, entry := range s {
			if entry.Q == 0 {
				continue
			}
			kind := offer.Match(entry)
			if kind == MatchNone {
				continue
			}
			if kind == MatchSuffix && !o.allowSuffix {
				continue
			}
			spec := entry.Specificity()
			switch {
			case entry.Q > bestQ:
				best, ok = offer, true
				bestQ = entry.Q
				bestSpec = spec
				bestKind = kind
				bestIdx = i
			case entry.Q < bestQ:
				// not better
			case spec > bestSpec:
				best, ok = offer, true
				bestSpec = spec
				bestKind = kind
				bestIdx = i
			case spec < bestSpec:
				// not better
			case kind > bestKind:
				best, ok = offer, true
				bestKind = kind
				bestIdx = i
			case kind < bestKind:
				// not better
			case bestIdx < 0 || i < bestIdx:
				best, ok = offer, true
				bestIdx = i
			}
		}
	}

	return best, ok
}

// splitTopLevel splits s on top-level commas, respecting double-quoted
// strings (RFC 7230 §3.2.6 — quoted-string).
func splitTopLevel(s string) []string {
	if strings.IndexByte(s, ',') < 0 {
		if t := strings.TrimSpace(s); t != "" {
			return []string{t}
		}
		return nil
	}
	var out []string
	start := 0
	inQuote := false
	escape := false
	for i := range len(s) {
		c := s[i]
		switch {
		case escape:
			escape = false
		case inQuote && c == '\\':
			escape = true
		case c == '"':
			inQuote = !inQuote
		case c == ',' && !inQuote:
			if t := strings.TrimSpace(s[start:i]); t != "" {
				out = append(out, t)
			}
			start = i + 1
		}
	}
	if t := strings.TrimSpace(s[start:]); t != "" {
		out = append(out, t)
	}

	return out
}
