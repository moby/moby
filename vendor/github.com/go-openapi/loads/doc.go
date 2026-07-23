// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package loads provides document loading methods for swagger (OAI v2) API specifications.
//
// It is used by other go-openapi packages to load and run analysis on local or remote spec documents.
//
// Loaders support JSON and YAML documents.
//
// # Security
//
// This package does not enforce a security policy of its own: like the underlying
// [github.com/go-openapi/swag/loading] utilities, it reads whatever the configured loader is
// allowed to read.
//
// When a spec — its path or its contents — may derive from untrusted input, the caller must confine loading explicitly.
//
// This is a deliberate design choice.
// Both this package and the [github.com/go-openapi/swag/loading] utilities are base building blocks:
// deciding which sources are legitimate, and containing access to them,
// requires application context that a general-purpose loader does not have.
//
// Just as sanitizing a file name before handing it to [os.ReadFile] is the caller's
// responsibility and not that function's, sanitizing and containing the path and references
// resolved here is the responsibility of the code that may feed them untrusted input.
//
// There are two distinct attack surfaces:
//
//   - The path passed to [Spec], [JSONSpec], or [Embedded]. By default a local path is read
//     with no confinement, so a caller-controlled path (including an absolute path or a
//     "file:///etc/passwd" URI) may read any file the process can access. A remote path is
//     fetched with [net/http.DefaultClient], which follows redirects and performs no
//     destination filtering, so a caller-controlled URL may reach internal services or cloud
//     metadata endpoints (server-side request forgery).
//
//   - The contents of the spec, when references are resolved. [Document.Expanded] follows the
//     "$ref" pointers found inside the document by calling the same loader recursively. A spec
//     obtained even from a trusted path can therefore drive arbitrary local reads
//     ("$ref": "file:///etc/passwd") or SSRF ("$ref": "http://169.254.169.254/...") through
//     its own contents. This amplification is specific to reference resolution and does not
//     exist in the raw loading utilities.
//
// Mitigation. Pass [github.com/go-openapi/swag/loading] options through [WithLoadingOptions];
// they are attached to the document's loader and so apply both to the initial load and to
// every "$ref" resolved during expansion:
//
//   - [github.com/go-openapi/swag/loading.WithRoot] confines local reads to a trusted
//     directory, rejecting absolute paths, ".." traversal, and symlinks that escape it. Prefer
//     it over a [github.com/go-openapi/swag/loading.WithFS] built from [os.DirFS], which does
//     not block symlink escapes.
//
//   - [github.com/go-openapi/swag/loading.WithHTTPClient] allows to supply a restricted HTTP client.
//     Enforce the network policy at dial time (a [net.Dialer] Control hook), so it also covers
//     redirects and DNS rebinding, which a URL-string allowlist cannot. See the example on
//     [Spec].
//
// Pre-baked loaders. When the opinionated defaults fit, [SpecRestricted], [JSONSpecRestricted]
// and [JSONDocRestricted] bundle a trusted root with a network-restricted client
// ([RestrictedHTTPClient]), and apply the confinement to "$ref" resolution as well — so the
// common case needs no manual wiring. To harden the global default in one call (so even callers
// that rely on the package-level loader are confined), use [SetRestrictedLoaders]. Reach for the
// options above when you need a custom policy; [IsForbiddenAddress] exposes the default network
// policy so you can reuse it as the base of your own HTTP client.
//
// Caveats:
//
//   - The package-level default loader (also installed as [github.com/go-openapi/spec.PathLoader])
//     carries no loading options and is therefore unconfined. It is used as a fallback when
//     expansion runs without a document loader, and by other go-openapi packages that resolve
//     references on their own. [AddLoader] does not fix this — it only prepends, leaving the
//     unconfined fallback reachable. Either build a confined loader per call, or replace the
//     global default outright with [SetLoaders] / [SetRestrictedLoaders].
//
//   - A custom loader installed via [WithDocLoader] or [AddLoader] only honors these
//     protections if its loading function actually applies the [github.com/go-openapi/swag/loading]
//     options it is given.
package loads
