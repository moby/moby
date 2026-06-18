// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mediatype

// Lookup finds the entry in m matching mediaType, with alias-aware
// fallback. It is the canonical seam for codec-map lookups in both
// the client and server runtimes — placing the fallback policy here
// keeps alias definitions (and any future lookup tolerances) in one
// place.
//
// Lookup tries the following, in order, returning the first hit:
//
//  1. mediaType verbatim (fast path for callers that already pass a
//     canonical, parameter-free string and store map keys in the
//     same form).
//  2. An alias-aware walk against the parsed "type/subtype" form:
//     a direct map hit on the parsed key, on its alias canonical
//     if any, and finally an O(len(m)) scan returning any map
//     entry whose key alias-canonicalizes to the same target.
//     Catches both "map keyed by canonical, query uses alias" and
//     "map keyed by one alias, query uses another alias of the
//     same canonical".
//  3. When [AllowSuffix] is passed in opts: the same alias-aware
//     walk against the RFC 6839 structured-syntax suffix base.
//     Catches the "spec/traffic divergence" case (request for
//     application/vnd.api+json finds a JSON consumer registered
//     under application/json). Query-side suffix fold only — no
//     map-side suffix folding.
//
// Lookup does NOT fall back to "*/*". Callers that want wildcard
// behavior (the historical resolveConsumer pattern in the client
// runtime) chain that themselves after a Lookup miss — keeping
// wildcard semantics explicit at each call site.
//
// Map keys are expected in canonical "type/subtype" form (no
// parameters). The runtime's default Consumers / Producers maps
// follow this convention.
//
// Returns (zero, false) when:
//
//   - m is empty;
//   - mediaType fails to parse and is not present verbatim;
//   - none of the active steps hits.
//
// The malformed-vs-not-found distinction is intentionally elided:
// codec-lookup callers treat both as the same "no codec" error path.
func Lookup[T any](m map[string]T, mediaType string, opts ...MatchOption) (T, bool) {
	var zero T
	if len(m) == 0 {
		return zero, false
	}
	o := applyMatchOptions(opts)
	// Fast path: raw key (preserves any caller behaviour that stored
	// non-canonical strings as map keys, and skips parsing in the
	// common already-canonical case).
	if v, ok := m[mediaType]; ok {
		return v, true
	}
	mt, err := Parse(mediaType)
	if err != nil {
		return zero, false
	}
	key := mt.Type + "/" + mt.Subtype
	if v, ok := findByCanonical(m, key); ok {
		return v, true
	}
	if o.allowSuffix && mt.Suffix != "" {
		base := mt.Base()
		if baseKey := base.Type + "/" + base.Subtype; baseKey != key {
			if v, ok := findByCanonical(m, baseKey); ok {
				return v, true
			}
		}
	}
	return zero, false
}

// findByCanonical returns the first entry in m whose key
// alias-canonicalizes to the same value as target.
//
// Tries direct hits before the O(len(m)) walk:
//
//  1. m[target] — map keyed by the same string.
//  2. m[aliases[target]] — map keyed by the canonical when target
//     is an alias.
//  3. Walk m: return any entry where canonical(k) == canonical(target).
//     Catches the "map keyed by an alias different from the query
//     side" case (e.g. registered under text/yaml, queried as
//     application/x-yaml — both canonicalize to application/yaml).
//
// Map size is single-digit for the runtime's codec maps, so the
// walk is negligible.
func findByCanonical[T any](m map[string]T, target string) (T, bool) {
	if v, ok := m[target]; ok {
		return v, true
	}
	canonTarget := target
	if canon, ok := aliases[target]; ok {
		canonTarget = canon
		if v, ok := m[canonTarget]; ok {
			return v, true
		}
	}
	for k, v := range m {
		kCanon := k
		if c, ok := aliases[k]; ok {
			kCanon = c
		}
		if kCanon == canonTarget {
			return v, true
		}
	}
	var zero T
	return zero, false
}
