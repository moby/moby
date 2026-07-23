# Loads OAI specs

<!-- Badges: status  -->
[![Tests][test-badge]][test-url] [![Coverage][cov-badge]][cov-url] [![CI vuln scan][vuln-scan-badge]][vuln-scan-url] [![CodeQL][codeql-badge]][codeql-url]
<!-- Badges: release & docker images  -->
<!-- Badges: code quality  -->
<!-- Badges: license & compliance -->
[![Release][release-badge]][release-url] [![Go Report Card][gocard-badge]][gocard-url] [![CodeFactor Grade][codefactor-badge]][codefactor-url] [![License][license-badge]][license-url]
<!-- Badges: documentation & support -->
<!-- Badges: others & stats -->
[![GoDoc][godoc-badge]][godoc-url] [![Discord Channel][discord-badge]][discord-url] [![go version][goversion-badge]][goversion-url] ![Top language][top-badge] ![Commits since latest release][commits-badge]

---

Loads OAI v2 API specification documents from local or remote locations.

Supports JSON and YAML documents.

## Announcements

* **2025-12-19** : new community chat on discord
  * a new discord community channel is available to be notified of changes and support users

You may join the discord community by clicking the invite link on the discord badge (also above). [![Discord Channel][discord-badge]][discord-url]

## Status

API is stable.

## Import this library in your project

```cmd
go get github.com/go-openapi/loads
```

## Basic usage

```go
  import (
	  "github.com/go-openapi/loads"
  )

  ...

	// loads a YAML spec from a http file
	doc, err := loads.Spec(ts.URL)
  
  ...

  // retrieves the object model for the API specification
  spec := doc.Spec()

  ...
```

See also the provided [examples](https://pkg.go.dev/github.com/go-openapi/loads#pkg-examples).

## Security

This library does not enforce a security policy of its own: it reads whatever the configured
loader is allowed to read.

This is deliberate — like `go-openapi/swag/loading`, it is a base utility,
and sanitizing or containing untrusted input is the caller's responsibility,
just as sanitizing a file name before passing it to `os.ReadFile` is not that function's job.

When a spec — its path or its `$ref` contents — may come from an untrusted source, confine
loading explicitly (e.g. `loading.WithRoot` for local files and a restricted
`loading.WithHTTPClient` for remote URLs, passed via `loads.WithLoadingOptions`).

For the common case, the pre-baked `loads.SpecRestricted` / `loads.JSONSpecRestricted` loaders
bundle a trusted root with a network-restricted client (`loads.RestrictedHTTPClient`) and apply
the confinement to `$ref` resolution as well:

```go
doc, err := loads.SpecRestricted(path, trustedRoot)
```

To harden the package-level default in one call — so even callers that rely on the global
loader (including cross-package `$ref` resolution via `spec.PathLoader`) are confined, with no
unconfined fallback left — use `loads.SetRestrictedLoaders` at startup:

```go
loads.SetRestrictedLoaders(trustedRoot)
```

Note that `loads.AddLoader` only *prepends* to the default chain, leaving the unconfined loader
reachable; use `loads.SetLoaders` / `loads.SetRestrictedLoaders` to replace it.

See the [Security section of the package documentation][security-doc] for the threat model and
runnable examples. For the project's vulnerability reporting policy, see [SECURITY.md](./SECURITY.md).

## Change log

See <https://github.com/go-openapi/loads/releases>

## Licensing

This library ships under the [SPDX-License-Identifier: Apache-2.0](./LICENSE).

## Other documentation

* [All-time contributors](./CONTRIBUTORS.md)
* [Contributing guidelines][contributing-doc-site]
* [Maintainers documentation][maintainers-doc-site]
* [Code style][style-doc-site]

## Cutting a new release

Maintainers can cut a new release by either:

* running [this workflow](https://github.com/go-openapi/loads/actions/workflows/bump-release.yml)
* or pushing a semver tag
  * signed tags are preferred
  * The tag message is prepended to release notes

<!-- Badges: status  -->
[test-badge]: https://github.com/go-openapi/loads/actions/workflows/go-test.yml/badge.svg
[test-url]: https://github.com/go-openapi/loads/actions/workflows/go-test.yml
[cov-badge]: https://codecov.io/gh/go-openapi/loads/branch/master/graph/badge.svg
[cov-url]: https://codecov.io/gh/go-openapi/loads
[vuln-scan-badge]: https://github.com/go-openapi/loads/actions/workflows/scanner.yml/badge.svg
[vuln-scan-url]: https://github.com/go-openapi/loads/actions/workflows/scanner.yml
[codeql-badge]: https://github.com/go-openapi/loads/actions/workflows/codeql.yml/badge.svg
[codeql-url]: https://github.com/go-openapi/loads/actions/workflows/codeql.yml
<!-- Badges: release & docker images  -->
[release-badge]: https://badge.fury.io/gh/go-openapi%2Floads.svg
[release-url]: https://badge.fury.io/gh/go-openapi%2Floads
<!-- Badges: code quality  -->
[gocard-badge]: https://goreportcard.com/badge/github.com/go-openapi/loads
[gocard-url]: https://goreportcard.com/report/github.com/go-openapi/loads
[codefactor-badge]: https://img.shields.io/codefactor/grade/github/go-openapi/loads
[codefactor-url]: https://www.codefactor.io/repository/github/go-openapi/loads
<!-- Badges: documentation & support -->
[godoc-badge]: https://pkg.go.dev/badge/github.com/go-openapi/loads
[godoc-url]: http://pkg.go.dev/github.com/go-openapi/loads
[discord-badge]: https://img.shields.io/discord/1446918742398341256?logo=discord&label=discord&color=blue
[discord-url]: https://discord.gg/FfnFYaC3k5

<!-- Badges: license & compliance -->
[license-badge]: http://img.shields.io/badge/license-Apache%20v2-orange.svg
[license-url]: https://github.com/go-openapi/loads/?tab=Apache-2.0-1-ov-file#readme
<!-- Badges: others & stats -->
[goversion-badge]: https://img.shields.io/github/go-mod/go-version/go-openapi/loads
[goversion-url]: https://github.com/go-openapi/loads/blob/master/go.mod
[top-badge]: https://img.shields.io/github/languages/top/go-openapi/loads
[commits-badge]: https://img.shields.io/github/commits-since/go-openapi/loads/latest
<!-- Documentation links -->
[security-doc]: https://pkg.go.dev/github.com/go-openapi/loads#hdr-Security
<!-- Organization docs -->
[contributing-doc-site]: https://go-openapi.github.io/doc-site/contributing/contributing/index.html
[maintainers-doc-site]: https://go-openapi.github.io/doc-site/maintainers/index.html
[style-doc-site]: https://go-openapi.github.io/doc-site/contributing/style/index.html
