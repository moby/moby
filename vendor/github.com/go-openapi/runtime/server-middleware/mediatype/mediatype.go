// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mediatype

import (
	"fmt"
	"mime"
	"strconv"
	"strings"
)

const wildcard = "*"

// Internal constants for the suffixBase table and any future
// in-package references to the well-known base media types.
const (
	typeApplication = "application"
	subtypeJSON     = "json"
	subtypeXML      = "xml"
	subtypeYAML     = "yaml"

	mtYAML = typeApplication + "/" + subtypeYAML
)

// Specificity scores returned by [MediaType.Specificity], ordered from
// least to most specific.
const (
	SpecificityAny             = iota // "*/*"
	SpecificityType                   // "type/*"
	SpecificityExact                  // "type/subtype" (no params)
	SpecificityExactWithParams        // "type/subtype;k=v"
)

// MatchKind classifies the strength of a match between two media
// types. Larger values represent stronger matches and win in
// negotiation tie-breaks.
//
// MatchExact covers direct subtype or wildcard agreement under RFC
// 7231 rules; MatchAlias is returned when the strict comparison
// fails but the two values agree after canonicalization through the
// internal alias table (see [MediaType.Canonical]); MatchSuffix is
// returned only when both alias and exact comparisons fail but the
// two values agree after folding the RFC 6839 structured-syntax
// suffix (see [MediaType.Base]).
//
// MatchSuffix matches are off by default at the negotiation /
// lookup callers — they count only when [AllowSuffix] is passed to
// [Set.BestMatch], [MatchFirst], or [Lookup]. The opt-in is the
// single user-visible knob; [MediaType.Match] itself always returns
// the strongest tier that succeeds.
type MatchKind int

// MatchKind values. Returned by [MediaType.Match].
const (
	MatchNone   MatchKind = iota // no match
	MatchSuffix                  // matched via the RFC 6839 suffix base
	MatchAlias                   // matched via the alias table
	MatchExact                   // matched directly (RFC 7231 semantics)
)

// MatchOption configures the matching tolerances used by
// [Set.BestMatch], [MatchFirst], and [Lookup]. The zero behaviour
// is strict: only [MatchAlias] and [MatchExact] count.
type MatchOption func(*matchOptions)

type matchOptions struct {
	allowSuffix bool
}

func applyMatchOptions(opts []MatchOption) matchOptions {
	var o matchOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// AllowSuffix returns a [MatchOption] that lets the caller count
// [MatchSuffix] results as valid matches. Use this to opt into
// RFC 6839 structured-syntax suffix tolerance for situations where
// the client/server traffic does not strictly abide by the spec
// (typical example: server returning application/problem+json
// against operations that only declare application/json in
// produces).
func AllowSuffix() MatchOption {
	return func(o *matchOptions) {
		o.allowSuffix = true
	}
}

type mediaTypeError string

func (e mediaTypeError) Error() string {
	return string(e)
}

// ErrMalformed is the sentinel returned (wrapped) by [Parse] when its input
// cannot be parsed as an RFC 7231 media type.
//
// Callers can test for it with [errors.Is] to distinguish a client-side
// malformed Content-Type header (an HTTP 400 outcome) from a well-formed
// value that simply matches no allowed entry (an HTTP 415 outcome).
const ErrMalformed mediaTypeError = "mediatype: malformed"

// MediaType is a parsed RFC 7231 media type with optional parameters and
// an optional q-value (used by Accept negotiation).
//
// Type, Subtype and the keys of Params are lowercased. Parameter values
// are preserved verbatim; comparisons are case-insensitive (matching the
// pre-v0.30 behaviour and the common convention for charset, version, etc.).
//
// Suffix exposes the RFC 6839 structured syntax suffix (the token after
// the final '+' in Subtype) as a parallel hint. Subtype itself retains
// the full wire value, so existing callers comparing Subtype against a
// string see no change.
type MediaType struct {
	Type    string
	Subtype string
	Suffix  string
	Params  map[string]string
	Q       float64
}

// suffixBase maps a known RFC 6839 / RFC 9512 structured syntax
// suffix (without the leading '+', lowercased) to its base media
// type. It is the authoritative table consulted by [MediaType.Base].
//
// The table is intentionally small: only suffixes whose base type
// has a codec in the default runtime maps are listed. CBOR, zip,
// BER, DER, FastInfoset and WBXML are registered by IANA but have
// no default codec in this runtime; adding them is gated on having
// something to do with them.
//
// Package-internal by design: the external API is [MediaType.Base].
// If users ever need to extend the table, a Register-style function
// is the right answer, not an exported mutable map.
var suffixBase = map[string]MediaType{
	subtypeJSON: {Type: typeApplication, Subtype: subtypeJSON},
	subtypeXML:  {Type: typeApplication, Subtype: subtypeXML},
	subtypeYAML: {Type: typeApplication, Subtype: subtypeYAML},
}

// aliases maps a deprecated or legacy media-type name to its
// canonical registered equivalent. Keys are the lowercased
// "type/subtype" form with no parameters; values are the canonical
// "type/subtype" form, also without parameters.
//
// Entries are limited to media types whose authoritative RFC
// explicitly names the alias. The seed entries cite RFC 9512 §2.1,
// which enumerates "Deprecated alias names for this type:
// application/x-yaml, text/yaml, and text/x-yaml" as part of the
// IANA registration template for application/yaml.
//
// Pull requests adding entries need an analogous citation in the
// commit message; entries without authoritative backing belong in
// caller-side canonicalization, not here.
//
// Package-internal by design: the external API is
// [MediaType.Canonical] and [MediaType.Match]. If users ever need
// to register their own aliases, a Register-style function is the
// right answer, not an exported mutable map.
var aliases = map[string]string{
	"application/x-yaml": mtYAML, // RFC 9512 §2.1
	"text/yaml":          mtYAML, // RFC 9512 §2.1
	"text/x-yaml":        mtYAML, // RFC 9512 §2.1
}

// Parse parses a single media type. The input may carry parameters and a
// q-value; the q-value is extracted into [MediaType.Q] and removed from
// [MediaType.Params].
//
// An empty input returns an error.
func Parse(s string) (MediaType, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return MediaType{}, fmt.Errorf("%w: empty value", ErrMalformed)
	}
	full, params, err := mime.ParseMediaType(s)
	if err != nil {
		return MediaType{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	slash := strings.IndexByte(full, '/')
	if slash <= 0 || slash == len(full)-1 {
		return MediaType{}, fmt.Errorf("%w: %q has no subtype", ErrMalformed, s)
	}
	mt := MediaType{
		Type:    full[:slash],
		Subtype: full[slash+1:],
		Q:       1.0,
	}
	// RFC 6839: structured syntax suffix is the trailing '+'-delimited
	// token of the subtype. Only the last '+' counts ("foo+bar+json" →
	// suffix "json"). A trailing '+' with nothing after it is not a
	// valid suffix and is ignored. mime.ParseMediaType has already
	// lowercased the subtype, so no further ToLower is needed.
	if plus := strings.LastIndexByte(mt.Subtype, '+'); plus >= 0 && plus < len(mt.Subtype)-1 {
		mt.Suffix = mt.Subtype[plus+1:]
	}
	if q, ok := params["q"]; ok {
		if qf, perr := strconv.ParseFloat(q, 64); perr == nil {
			if qf < 0 {
				qf = 0
			}
			if qf > 1 {
				qf = 1
			}
			mt.Q = qf
		}
		delete(params, "q")
	}
	if len(params) > 0 {
		mt.Params = params
	}

	return mt, nil
}

// String renders the canonical "type/subtype;k=v;k=v" form. Parameters are
// emitted in lexicographic key order (the standard library guarantees this)
// so the result is stable. The q-value is NOT emitted — it is meta, not
// part of the media type identity.
func (m MediaType) String() string {
	if m.Type == "" && m.Subtype == "" {
		return ""
	}

	return mime.FormatMediaType(m.Type+"/"+m.Subtype, m.Params)
}

// Matches reports whether the receiver accepts other, per the package
// documentation: the receiver is the bound, other is the constraint.
func (m MediaType) Matches(other MediaType) bool {
	if !typeAgrees(m.Type, other.Type) {
		return false
	}
	if !subtypeAgrees(m.Type, m.Subtype, other.Type, other.Subtype) {
		return false
	}
	if len(m.Params) == 0 {
		return true
	}
	for k, v := range other.Params {
		sv, ok := m.Params[k]
		if !ok || !strings.EqualFold(sv, v) {
			return false
		}
	}

	return true
}

// Specificity returns a numeric score for ordering matches. Higher is more
// specific. The returned value is one of [SpecificityAny],
// [SpecificityType], [SpecificityExact] or [SpecificityExactWithParams].
func (m MediaType) Specificity() int {
	if m.Type == wildcard && m.Subtype == wildcard {
		return SpecificityAny
	}
	if m.Subtype == wildcard {
		return SpecificityType
	}
	if len(m.Params) == 0 {
		return SpecificityExact
	}

	return SpecificityExactWithParams
}

// typeAgrees reports whether two top-level types match, allowing "*" on
// either side. A type of "*" without a "*" subtype is rejected per RFC
// 7231 §5.3.2 ("*/sub" is not valid), but Parse never produces such a
// shape — it goes through mime.ParseMediaType.
func typeAgrees(a, b string) bool {
	return a == wildcard || b == wildcard || a == b
}

// subtypeAgrees handles the "type/*" wildcard: the bare type must match
// (a "*/*" pair has already been accepted by typeAgrees above).
func subtypeAgrees(at, asub, bt, bsub string) bool {
	if at == wildcard || bt == wildcard {
		// at least one side is "*/*" or "*/sub". With typeAgrees having
		// returned true, we accept.
		return true
	}
	if asub == wildcard || bsub == wildcard {
		return true
	}

	return asub == bsub
}

// StripParams returns a copy of m with no parameters. Q is preserved
// because it drives negotiation ordering, not media-type identity.
//
// Useful for the legacy "ignore parameters" negotiation mode.
func (m MediaType) StripParams() MediaType {
	return MediaType{Type: m.Type, Subtype: m.Subtype, Suffix: m.Suffix, Q: m.Q}
}

// Base returns the base media type implied by the RFC 6839 structured
// syntax suffix, or m unchanged when:
//
//   - Suffix is empty;
//   - Suffix is not present in the package-internal suffix→base table.
//
// The returned value represents the structural base only: it carries
// no parameters and no q-value. Use it to find a codec for the
// underlying wire format — for example, "application/vnd.api+json"
// resolves to "application/json".
//
// Base does not mutate the receiver.
func (m MediaType) Base() MediaType {
	if m.Suffix == "" {
		return m
	}
	base, ok := suffixBase[m.Suffix]
	if !ok {
		return m
	}
	return base
}

// Canonical returns m rewritten to its canonical media type via
// the package-internal alias table, or m unchanged when
// (Type, Subtype) is not a known alias. Params and Q are preserved on the returned value; Suffix
// is recomputed from the canonical Subtype (none of the current
// entries carry a suffix, but the contract is forward-safe).
//
// Canonical does not mutate the receiver.
func (m MediaType) Canonical() MediaType {
	key := m.Type + "/" + m.Subtype
	canon, ok := aliases[key]
	if !ok {
		return m
	}
	slash := strings.IndexByte(canon, '/')
	out := m
	out.Type = canon[:slash]
	out.Subtype = canon[slash+1:]
	out.Suffix = ""
	if plus := strings.LastIndexByte(out.Subtype, '+'); plus >= 0 && plus < len(out.Subtype)-1 {
		out.Suffix = out.Subtype[plus+1:]
	}
	return out
}

// Match reports how m matches other, classifying the result by
// [MatchKind]. Used by negotiation to rank candidate offers:
// stronger tiers win when both apply.
//
// Returns, strongest first:
//
//   - MatchExact when m.Matches(other) is true under the strict
//     RFC 7231 rules (including wildcards and the param subset
//     rule).
//   - MatchAlias when m.Canonical().Matches(other.Canonical())
//     is true but the strict comparison failed.
//   - MatchSuffix when m.Base().Canonical().Matches(
//     other.Base().Canonical()) is true but the alias comparison
//     failed (RFC 6839 structured-syntax suffix fold).
//   - MatchNone otherwise.
//
// The asymmetric "bound vs constraint" rule of [MediaType.Matches]
// is preserved at every tier. Match itself is always lenient — the
// opt-in to count MatchSuffix lives one level up at [Set.BestMatch],
// [MatchFirst], and [Lookup] via [AllowSuffix].
func (m MediaType) Match(other MediaType) MatchKind {
	if m.Matches(other) {
		return MatchExact
	}
	if m.Canonical().Matches(other.Canonical()) {
		return MatchAlias
	}
	if m.Base().Canonical().Matches(other.Base().Canonical()) {
		return MatchSuffix
	}
	return MatchNone
}
