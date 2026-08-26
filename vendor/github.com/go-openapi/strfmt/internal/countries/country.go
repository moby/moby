// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:generate go run gen.go -- countries.go
package countries

// Country identifies a country with its english name, and ISO3166 codes.
//
//nolint:tagliatelle // JSON tags mirror the iso3166.json source keys (alpha-3, alpha-2, country-code); renaming them breaks the embedded-JSON unmarshal.
type Country struct {
	Name      string `json:"name"`
	ISOAlpha3 string `json:"alpha-3"`
	ISOAlpha2 string `json:"alpha-2"`
	Code      string `json:"country-code"`
}
