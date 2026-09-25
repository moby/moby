// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package mediatype provides a typed value for media types
// defined by RFC 7231 and RFC 2045.
//
// The matching/selection primitives used by both server-side
// validation and Accept-header negotiation.
//
// The package is stdlib-only.
//
// # The matching rule
//
// [MediaType.Matches] is asymmetric. The receiver acts as the "bound"
// (an allowed entry on the server side, or a candidate offer when
// matching against an Accept entry); the argument is the constraint
// (the actual incoming request, or the Accept entry being satisfied).
//
//   - bare type/subtype must agree, with wildcard handling on either
//     side ("*/*" matches anything; "type/*" matches any subtype);
//   - if the receiver carries no parameters, any constraint is
//     accepted regardless of its parameters;
//   - otherwise every (key,value) pair on the constraint must be
//     present on the receiver, with case-insensitive value
//     comparison. The receiver may carry additional parameters the
//     constraint does not list.
//
// q-values are NOT considered by [MediaType.Matches] — they are the
// negotiator's concern, handled inside [Set.BestMatch].
package mediatype
