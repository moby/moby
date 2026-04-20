# runtime

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

A runtime for go OpenAPI toolkit.

The runtime component for use in code generation or as untyped usage.

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
go get github.com/go-openapi/runtime
```

## Change log

See <https://github.com/go-openapi/runtime/releases>

For pre-v0.30.0 releases see [release notes](docs/NOTES.md).

**What coming next?**

Moving forward, we want to :

* [ ] continue narrowing down the scope of dependencies:
  * yaml support in an independent module
  * introduce more up-to-date support for opentelemetry as a separate module that evolves
    independently from the main package (to avoid breaking changes, the existing API
    will remain maintained, but evolve at a slower pace than opentelemetry).
* [ ] fix a few known issues with some file upload requests (e.g. #286)

## Licensing

This library ships under the [SPDX-License-Identifier: Apache-2.0](./LICENSE).

See the license [NOTICE](./NOTICE), which recalls the licensing terms of all the pieces of software
on top of which it has been built.

## Other documentation

* [FAQ](docs/FAQ.md)
* [All-time contributors](./CONTRIBUTORS.md)
* [Contributing guidelines](.github/CONTRIBUTING.md)
* [Maintainers documentation](docs/MAINTAINERS.md)
* [Code style](docs/STYLE.md)

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
[godoc-badge]: https://pkg.go.dev/badge/github.com/go-openapi/runtime
[godoc-url]: http://pkg.go.dev/github.com/go-openapi/runtime
[slack-logo]: https://a.slack-edge.com/e6a93c1/img/icons/favicon-32.png
[slack-badge]: https://img.shields.io/badge/slack-blue?link=https%3A%2F%2Fgoswagger.slack.com%2Farchives%2FC04R30YM
[slack-url]: https://goswagger.slack.com/archives/C04R30YMU
[discord-badge]: https://img.shields.io/discord/1446918742398341256?logo=discord&label=discord&color=blue
[discord-url]: https://discord.gg/twZ9BwT3

<!-- Badges: license & compliance -->
[license-badge]: http://img.shields.io/badge/license-Apache%20v2-orange.svg
[license-url]: https://github.com/go-openapi/runtime/?tab=Apache-2.0-1-ov-file#readme
<!-- Badges: others & stats -->
[goversion-badge]: https://img.shields.io/github/go-mod/go-version/go-openapi/runtime
[goversion-url]: https://github.com/go-openapi/runtime/blob/master/go.mod
[top-badge]: https://img.shields.io/github/languages/top/go-openapi/runtime
[commits-badge]: https://img.shields.io/github/commits-since/go-openapi/runtime/latest
