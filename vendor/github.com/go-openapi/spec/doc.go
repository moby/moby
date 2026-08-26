// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package spec exposes an object model for OpenAPIv2 specifications (swagger).
//
// The exposed data structures know how to serialize to and deserialize from JSON.
//
// # Security
//
// Resolving and expanding "$ref" pointers loads documents through a pluggable loader (see
// [ExpandOptions.PathLoader] and [ExpandOptions.PathLoaderWithOptions]). By default, that
// loader is NOT sandboxed, so a specification obtained from an untrusted source can abuse it:
//
//   - A local "$ref" such as "file:///etc/passwd" or a relative "../../secret.json" is read
//     straight off disk. A malicious specification can therefore read any file the process can
//     access (arbitrary file read / path traversal, CWE-22).
//   - A remote "$ref" such as "http://169.254.169.254/..." is fetched with no restriction. A
//     malicious specification can therefore probe or reach internal addresses (SSRF, CWE-918).
//
// Do NOT expand or resolve an untrusted specification with the default options. To process
// untrusted specifications safely, inject a confined loader:
//
//   - Recommended: use the restricted loaders from github.com/go-openapi/loads, for example
//     loads.SpecRestricted(path, root) or loads.SetRestrictedLoaders(root). They confine local
//     reads to root and route remote fetches through a client that rejects loopback, private and
//     link-local addresses, and the confinement applies to every "$ref" resolved during
//     expansion.
//   - Or directly: set [ExpandOptions.PathLoaderWithOptions] to a loader built with
//     github.com/go-openapi/swag/loading options such as loading.WithRoot (to confine local
//     reads to a directory) and loading.WithHTTPClient (to restrict remote fetches). A "$ref"
//     that resolves outside root is then rejected, including one reached through a "file://"
//     URI or a "../" traversal.
//
// Expanding an untrusted specification also has a resource-exhaustion vector ("$ref"
// amplification); see [ExpandOptions.MaxExpansionNodes], which is bounded by default.
package spec
