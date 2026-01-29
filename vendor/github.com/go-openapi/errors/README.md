# errors

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

Shared errors and error interface used throughout the various libraries found in the go-openapi toolkit.

## Announcements

* **2025-12-19** : new community chat on discord
  * a new discord community channel is available to be notified of changes and support users
  * our venerable Slack channel remains open, and will be eventually discontinued on **2026-03-31**

You may join the discord community by clicking the invite link on the discord badge (also above). [![Discord Channel][discord-badge]][discord-url]

Or join our Slack channel: [![Slack Channel][slack-logo]![slack-badge]][slack-url]

## Status

API is stable.

## Import this library in your project

```cmd
go get github.com/go-openapi/errors
```

## Basic usage

```go
const url = "https://www.example.com/#"

errGeneric := New(401,"onvalid argument: %s", url)

errNotFound := NotFound("resource not found: %s", url)

errNotImplemented := NotImplemented("method: %s", url)
```

## Change log

See <https://github.com/go-openapi/errors/releases>

<!--
## References
-->

## Licensing

This library ships under the [SPDX-License-Identifier: Apache-2.0](./LICENSE).

<!--
See the license [NOTICE](./NOTICE), which recalls the licensing terms of all the pieces of software
on top of which it has been built.
-->

<!--
## Limitations
-->

## Other documentation

* [All-time contributors](./CONTRIBUTORS.md)
* [Contributing guidelines](.github/CONTRIBUTING.md)
* [Maintainers documentation](docs/MAINTAINERS.md)
* [Code style](docs/STYLE.md)

## Cutting a new release

Maintainers can cut a new release by either:

* running [this workflow](https://github.com/go-openapi/errors/actions/workflows/bump-release.yml)
* or pushing a semver tag
  * signed tags are preferred
  * The tag message is prepended to release notes

<!-- Badges: status  -->
[test-badge]: https://github.com/go-openapi/errors/actions/workflows/go-test.yml/badge.svg
[test-url]: https://github.com/go-openapi/errors/actions/workflows/go-test.yml
[cov-badge]: https://codecov.io/gh/go-openapi/errors/branch/master/graph/badge.svg
[cov-url]: https://codecov.io/gh/go-openapi/errors
[vuln-scan-badge]: https://github.com/go-openapi/errors/actions/workflows/scanner.yml/badge.svg
[vuln-scan-url]: https://github.com/go-openapi/errors/actions/workflows/scanner.yml
[codeql-badge]: https://github.com/go-openapi/errors/actions/workflows/codeql.yml/badge.svg
[codeql-url]: https://github.com/go-openapi/errors/actions/workflows/codeql.yml
<!-- Badges: release & docker images  -->
[release-badge]: https://badge.fury.io/gh/go-openapi%2Ferrors.svg
[release-url]: https://badge.fury.io/gh/go-openapi%2Ferrors
[gomod-badge]: https://badge.fury.io/go/github.com%2Fgo-openapi%2Ferrors.svg
[gomod-url]: https://badge.fury.io/go/github.com%2Fgo-openapi%2Ferrors
<!-- Badges: code quality  -->
[gocard-badge]: https://goreportcard.com/badge/github.com/go-openapi/errors
[gocard-url]: https://goreportcard.com/report/github.com/go-openapi/errors
[codefactor-badge]: https://img.shields.io/codefactor/grade/github/go-openapi/errors
[codefactor-url]: https://www.codefactor.io/repository/github/go-openapi/errors
<!-- Badges: documentation & support -->
[doc-badge]: https://img.shields.io/badge/doc-site-blue?link=https%3A%2F%2Fgoswagger.io%2Fgo-openapi%2F
[doc-url]: https://goswagger.io/go-openapi
[godoc-badge]: https://pkg.go.dev/badge/github.com/go-openapi/errors
[godoc-url]: http://pkg.go.dev/github.com/go-openapi/errors
[slack-logo]: https://a.slack-edge.com/e6a93c1/img/icons/favicon-32.png
[slack-badge]: https://img.shields.io/badge/slack-blue?link=https%3A%2F%2Fgoswagger.slack.com%2Farchives%2FC04R30YM
[slack-url]: https://goswagger.slack.com/archives/C04R30YMU
[discord-badge]: https://img.shields.io/discord/1446918742398341256?logo=discord&label=discord&color=blue
[discord-url]: https://discord.gg/DrafRmZx

<!-- Badges: license & compliance -->
[license-badge]: http://img.shields.io/badge/license-Apache%20v2-orange.svg
[license-url]: https://github.com/go-openapi/errors/?tab=Apache-2.0-1-ov-file#readme
<!-- Badges: others & stats -->
[goversion-badge]: https://img.shields.io/github/go-mod/go-version/go-openapi/errors
[goversion-url]: https://github.com/go-openapi/errors/blob/master/go.mod
[top-badge]: https://img.shields.io/github/languages/top/go-openapi/errors
[commits-badge]: https://img.shields.io/github/commits-since/go-openapi/errors/latest
