// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mediatype

// MatchFirst reports whether actual matches any entry in allowed,
// using [MediaType.Match] — the param-aware RFC 7231 rule plus the
// alias bridge from the package-internal alias table.
//
// The scan is multi-pass and tier-ordered: the first pass returns
// the first allowed entry that matches under [MatchExact] (RFC 7231
// semantics); the second pass looks for a [MatchAlias] match; when
// [AllowSuffix] is in opts a third pass looks for a [MatchSuffix]
// match. This preserves the "stronger tier wins" ordering from
// [Set.BestMatch] while keeping the "first match wins" semantics
// within each tier.
//
// Return values:
//
//   - (matched, true,  nil)        — the first allowed entry that
//     matches, with exact matches preferred over alias matches.
//   - (zero,    false, nil)        — actual is well-formed but no
//     allowed entry accepts it. Maps to an HTTP 415 outcome.
//   - (zero,    false, err)        — actual fails to parse. err
//     wraps [ErrMalformed], so callers can use [errors.Is] to
//     distinguish this case. Maps to an HTTP 400 outcome.
//
// Allowed entries that themselves fail to parse are skipped (they
// cannot match any well-formed actual), and no error is surfaced
// for them.
//
// An empty allowed list returns (zero, false, nil). MatchFirst is
// the primitive; callers decide what no-constraints means in their
// context.
func MatchFirst(allowed []string, actual string, opts ...MatchOption) (MediaType, bool, error) {
	if len(allowed) == 0 {
		return MediaType{}, false, nil
	}
	actualMT, err := Parse(actual)
	if err != nil {
		return MediaType{}, false, err
	}
	o := applyMatchOptions(opts)
	// Tier-ordered passes over the allowed list. The list is
	// typically short (an operation's Consumes set), so re-parsing
	// each entry on every pass is cheaper than caching parses across
	// passes.
	tiers := []MatchKind{MatchExact, MatchAlias}
	if o.allowSuffix {
		tiers = append(tiers, MatchSuffix)
	}
	for _, want := range tiers {
		for _, a := range allowed {
			allowedMT, perr := Parse(a)
			if perr != nil {
				continue
			}
			if allowedMT.Match(actualMT) == want {
				return allowedMT, true, nil
			}
		}
	}

	return MediaType{}, false, nil
}
