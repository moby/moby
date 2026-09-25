# runtime

<!-- Badges: status  -->
[![Tests][test-badge]][test-url] [![Coverage][cov-badge]][cov-url] [![CI vuln scan][vuln-scan-badge]][vuln-scan-url] [![CodeQL][codeql-badge]][codeql-url]
<!-- Badges: release & docker images  -->
<!-- Badges: code quality  -->
<!-- Badges: license & compliance -->
[![Release][release-badge]][release-url] [![Go Report Card][gocard-badge]][gocard-url] [![CodeFactor Grade][codefactor-badge]][codefactor-url] [![License][license-badge]][license-url]
<!-- Badges: documentation & support -->
<!-- Badges: others & stats -->
[![Doc][doc-badge]][doc-url] [![GoDoc][godoc-badge]][godoc-url] [![Discord Channel][discord-badge]][discord-url] [![go version][goversion-badge]][goversion-url] ![Top language][top-badge] ![Commits since latest release][commits-badge]
---

A runtime for go OpenAPI toolkit.

The runtime component for use in code generation or as untyped usage.

## Announcements

[**Complete documentation as github pages**][doc-url]

**Changes to the API surface in `v0.30.0`**:

* utility package `header` has now moved to `github.com/go-openapi/runtime/server-middleware/negotiate/header`

> A shim is provided to support existing programs, with a deprecation notice.

**Changes in semantics in `v0.30.0`**:

Function `negotiate.NegotiateContentType` (available as an alias for backward compatibility as `middleware.NegotiateContentType`
now performs a full match considering MIME parameters.

The previous behavior (matching in order of appearance after stripping parameters) may be enabled explicitly with
option `negotiate.WithIgnoreParameters`.

* **2026-05-07** : exposed UI and Spec middleware as a separate, dependency-free module.

> Newly available package: `github.com/go-openapi/runtime/server-middleware/docui` that now holds our
> UI and spec serve middleware.
>
> A shim is available in `github.com/go-openapi/runtime/middleware` to bridge the older UI options to the new ones,
> with a deprecation notice.
>
> Methods that were unduly exported and purely used to manipulate options (e.g. `SwaggerUIOpts.EnsureDefaults`) have been
> removed. New options in `docui` should be used instead.

> Users may reuse this middleware to serve a Redoc, Rapidoc or SwaggerUI documentation without
> importing the complete go-openapi scaffolding.

* **2026-05-05** : exposed content negotiation methods as a separate, dependency-free module

> Users may reuse these utilities to support content-negotiation without extra dependencies.
>
> Newly available module: `github.com/go-openapi/runtime/server-middleware`
>
> Newly available packages: `github.com/go-openapi/runtime/server-middleware/negotiate` and
> `github.com/go-openapi/runtime/server-middleware/mediatype`.

## Status

API is stable.

## Import this library in your project

```cmd
go get github.com/go-openapi/runtime
```

## Change log

See <https://github.com/go-openapi/runtime/releases>

For v0.29.0 release see [release notes](docs/NOTES.md).
From that release onwards, changes are tracked in the github release notes.

**What coming next?**

Moving forward, we want to :

* [x] fix a few known issues with some file upload requests (e.g. #286)
* [] continue narrowing down the scope of dependencies:
  * [x] split middleware and other useful utilities as a separate dependency-free module
  * yaml support in an independent module (v2)
  * introduce more up-to-date support for opentelemetry as a separate module that evolves
    independently from the main package (to avoid breaking changes, the existing API
    will remain maintained, but evolve at a slower pace than opentelemetry). (v2)
* [] publish proper documentation and examples

## Licensing

This library ships under the [SPDX-License-Identifier: Apache-2.0](./LICENSE).

See the license [NOTICE](./NOTICE), which recalls the licensing terms of all the pieces of software
on top of which it has been built.

## Other documentation

* [FAQ](https://go-openapi.github.io/runtime/tutorials/faq/) · [Media-type selection](https://go-openapi.github.io/runtime/tutorials/media-types/) · [Client keep-alive](https://go-openapi.github.io/runtime/tutorials/keep-alive/)
* [All-time contributors](./CONTRIBUTORS.md)
* [Contributing guidelines][contributing-doc-site]
* [Maintainers documentation][maintainers-doc-site]
* [Code style][style-doc-site]

## Cutting a new release

Maintainers can cut a new release by either:

* running [this workflow](https://github.com/go-openapi/runtime/actions/workflows/bump-release.yml)
* or pushing a semver tag
  * signed tags are preferred
  * The tag message is prepended to release notes

<!-- Badges: status  -->
[test-badge]: https://github.com/go-openapi/runtime/actions/workflows/go-test.yml/badge.svg
[test-url]: https://github.com/go-openapi/runtime/actions/workflows/go-test.yml
[cov-badge]: https://codecov.io/gh/go-openapi/runtime/branch/master/graph/badge.svg
[cov-url]: https://codecov.io/gh/go-openapi/runtime
[vuln-scan-badge]: https://github.com/go-openapi/runtime/actions/workflows/scanner.yml/badge.svg
[vuln-scan-url]: https://github.com/go-openapi/runtime/actions/workflows/scanner.yml
[codeql-badge]: https://github.com/go-openapi/runtime/actions/workflows/codeql.yml/badge.svg
[codeql-url]: https://github.com/go-openapi/runtime/actions/workflows/codeql.yml
<!-- Badges: release & docker images  -->
[release-badge]: https://badge.fury.io/gh/go-openapi%2Fruntime.svg
[release-url]: https://badge.fury.io/gh/go-openapi%2Fruntime
<!-- Badges: code quality  -->
[gocard-badge]: https://goreportcard.com/badge/github.com/go-openapi/runtime
[gocard-url]: https://goreportcard.com/report/github.com/go-openapi/runtime
[codefactor-badge]: https://img.shields.io/codefactor/grade/github/go-openapi/runtime
[codefactor-url]: https://www.codefactor.io/repository/github/go-openapi/runtime
<!-- Badges: documentation & support -->
[doc-badge]: https://img.shields.io/badge/doc-site-blue?link=https%3A%2F%2Fgo-openapi.github.io%2Fruntime%2F
[doc-url]: https://go-openapi.github.io/runtime
[godoc-badge]: https://pkg.go.dev/badge/github.com/go-openapi/runtime
[godoc-url]: http://pkg.go.dev/github.com/go-openapi/runtime
[discord-badge]: https://img.shields.io/discord/1446918742398341256?logo=discord&label=discord&color=blue
[discord-url]: https://discord.gg/FfnFYaC3k5

<!-- Badges: license & compliance -->
[license-badge]: http://img.shields.io/badge/license-Apache%20v2-orange.svg
[license-url]: https://github.com/go-openapi/runtime/?tab=Apache-2.0-1-ov-file#readme
<!-- Badges: others & stats -->
[goversion-badge]: https://img.shields.io/github/go-mod/go-version/go-openapi/runtime
[goversion-url]: https://github.com/go-openapi/runtime/blob/master/go.mod
[top-badge]: https://img.shields.io/github/languages/top/go-openapi/runtime
[commits-badge]: https://img.shields.io/github/commits-since/go-openapi/runtime/latest
<!-- Organization docs -->
[contributing-doc-site]: https://go-openapi.github.io/doc-site/contributing/contributing/index.html
[maintainers-doc-site]: https://go-openapi.github.io/doc-site/maintainers/index.html
[style-doc-site]: https://go-openapi.github.io/doc-site/contributing/style/index.html
