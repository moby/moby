// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package negotiate provides server-side HTTP content negotiation
// helpers — selecting the response Content-Type from an Accept header
// and the response Content-Encoding from an Accept-Encoding header.
//
// The package is stdlib-only (modulo the typed [mediatype.MediaType]
// values it consumes).
//
// The exported [ContentType] honours MIME-type parameters by default;
// use [WithIgnoreParameters] to restore the pre-v0.30 looser match.
package negotiate
