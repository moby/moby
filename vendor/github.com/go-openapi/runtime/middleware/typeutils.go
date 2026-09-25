// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package middleware

import "strings"

// normalizeOffer strips the parameter section (";...") from a media-type
// string.
func normalizeOffer(orig string) string {
	// NOTE(maintainers): Despite its name (kept for historical reasons), this helper is
	// not about Accept negotiation — it is used to derive the bare type that
	// keys the producer/consumer maps registered on a [RoutableAPI].
	// Those maps are looked up by the bare media type, so an entry registered as
	// "application/json" satisfies a route that declares "application/json;
	// charset=utf-8" and vice-versa.
	const maxParts = 2

	return strings.SplitN(orig, ";", maxParts)[0]
}

// normalizeOffers is the slice form of [normalizeOffer].
func normalizeOffers(orig []string) []string {
	norm := make([]string, 0, len(orig))
	for _, o := range orig {
		norm = append(norm, normalizeOffer(o))
	}

	return norm
}
