# strfmt

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

Golang support for string formats defined by JSON Schema and OpenAPI.

## Announcements

* **2026-03-07** : v0.26.0 **dropped dependency to the mongodb driver**
  * mongodb users can still use this package without any change
  * however, we have frozen the back-compatible support for mongodb driver at v2.5.0
  * users who want to keep-up with future evolutions (possibly incompatible) of this driver
    can do so by adding a blank import in their program: `import _ "github.com/go-openapi/strfmt/enable/mongodb"`.
    This will switch the behavior to the actual driver, which remains regularly updated as an independent module.

## Status

API is stable.

## Import this library in your project

```cmd
go get github.com/go-openapi/strfmt
```

## Contents

This package exposes a registry of data types to support string formats in the go-openapi toolkit.

`strfmt` represents a well known string format such as hostname or email.

This package provides a few extra formats such as credit card (US), color, etc.

Format types can serialize and deserialize JSON or from a SQL database.

BSON is also supported (MongoDB).

### Supported formats

`go-openapi/strfmt` follows the swagger 2.0 specification with the following formats
defined [here](https://github.com/OAI/OpenAPI-Specification/blob/master/versions/2.0.md#data-types).

It also provides convenient extensions to go-openapi users.

- [x] JSON-schema draft 4 formats
  - date-time
  - email
  - hostname
  - ipv4
  - ipv6
  - uri
- [x] swagger 2.0 format extensions
  - binary
  - byte (e.g. base64 encoded string)
  - date (e.g. "1970-01-01")
  - password
- [x] go-openapi custom format extensions
  - bsonobjectid (BSON objectID)
  - creditcard
  - duration (e.g. "3 weeks", "1ms")
  - hexcolor (e.g. "#FFFFFF")
  - isbn, isbn10, isbn13
  - mac (e.g "01:02:03:04:05:06")
  - rgbcolor (e.g. "rgb(100,100,100)")
  - ssn
  - uuid, uuid3, uuid4, uuid5, uuid7
  - cidr (e.g. "192.0.2.1/24", "2001:db8:a0b:12f0::1/32")
  - ulid (e.g. "00000PP9HGSBSSDZ1JTEXBJ0PW", [spec](https://github.com/ulid/spec))

> NOTE: as the name stands for, this package is intended to support string formatting only.
> It does not provide validation for numerical values with swagger format extension for JSON types "number" or
> "integer" (e.g. float, double, int32...).

### Type conversion

All types defined here are stringers and may be converted to strings with `.String()`.
Note that most types defined by this package may be converted directly to string like `string(Email{})`.

`Date` and `DateTime` may be converted directly to `time.Time` like `time.Time(Time{})`.
Similarly, you can convert `Duration` to `time.Duration` as in `time.Duration(Duration{})`

### Using pointers

The `conv` subpackage provides helpers to convert the types to and from pointers, just like `go-openapi/swag` does
with primitive types.

### Format types

Types defined in `strfmt` expose marshaling and validation capabilities.

List of defined types:
- Base64
- CreditCard
- Date
- DateTime
- Duration
- Email
- HexColor
- Hostname
- IPv4
- IPv6
- CIDR
- ISBN
- ISBN10
- ISBN13
- MAC
- ObjectId
- Password
- RGBColor
- SSN
- URI
- UUID
- [UUID3](https://www.rfc-editor.org/rfc/rfc9562.html#name-uuid-version-3)
- [UUID4](https://www.rfc-editor.org/rfc/rfc9562.html#name-uuid-version-4)
- [UUID5](https://www.rfc-editor.org/rfc/rfc9562.html#name-uuid-version-5)
- [UUID7](https://www.rfc-editor.org/rfc/rfc9562.html#name-uuid-version-7)
- [ULID](https://github.com/ulid/spec)

### Database support

All format types implement the `database/sql` interfaces `sql.Scanner` and `driver.Valuer`,
so they work out of the box with Go's standard `database/sql` package and any SQL driver.

All format types also implement BSON marshaling/unmarshaling for use with MongoDB.
By default, a built-in minimal codec is used (compatible with mongo-driver v2.5.0).
For full driver support, add `import _ "github.com/go-openapi/strfmt/enable/mongodb"`.

> **MySQL / MariaDB caveat for `DateTime`:**
> The `go-sql-driver/mysql` driver has hard-coded handling for `time.Time` but does not
> intercept type redefinitions like `strfmt.DateTime`. As a result, `DateTime.Value()` sends
> an RFC 3339 string (e.g. `"2024-06-15T12:30:45.123Z"`) that MySQL/MariaDB rejects for
> `DATETIME` columns.
>
> Workaround: set `strfmt.MarshalFormat` to a MySQL-compatible format such as
> `strfmt.ISO8601LocalTime` and normalize to UTC before marshaling:
>
> ```go
> strfmt.MarshalFormat = strfmt.ISO8601LocalTime
> strfmt.NormalizeTimeForMarshal = func(t time.Time) time.Time { return t.UTC() }
> ```
>
> See [#174](https://github.com/go-openapi/strfmt/issues/174) for details.

Integration tests for MongoDB, MariaDB, and PostgreSQL run in CI to verify database roundtrip
compatibility for all format types. See [`internal/testintegration/`](internal/testintegration/).

## Change log

See <https://github.com/go-openapi/strfmt/releases>

## References

<https://github.com/OAI/OpenAPI-Specification/blob/main/versions/2.0.md>

## Licensing

This library ships under the [SPDX-License-Identifier: Apache-2.0](./LICENSE).

## Other documentation

* [All-time contributors](./CONTRIBUTORS.md)
* [Contributing guidelines][contributing-doc-site]
* [Maintainers documentation][maintainers-doc-site]
* [Code style][style-doc-site]

## Cutting a new release

Maintainers can cut a new release by either:

* running [this workflow](https://github.com/go-openapi/strfmt/actions/workflows/bump-release.yml)
* or pushing a semver tag
  * signed tags are preferred
  * The tag message is prepended to release notes

<!-- Badges: status  -->
[test-badge]: https://github.com/go-openapi/strfmt/actions/workflows/go-test.yml/badge.svg
[test-url]: https://github.com/go-openapi/strfmt/actions/workflows/go-test.yml
[cov-badge]: https://codecov.io/gh/go-openapi/strfmt/branch/master/graph/badge.svg
[cov-url]: https://codecov.io/gh/go-openapi/strfmt
[vuln-scan-badge]: https://github.com/go-openapi/strfmt/actions/workflows/scanner.yml/badge.svg
[vuln-scan-url]: https://github.com/go-openapi/strfmt/actions/workflows/scanner.yml
[codeql-badge]: https://github.com/go-openapi/strfmt/actions/workflows/codeql.yml/badge.svg
[codeql-url]: https://github.com/go-openapi/strfmt/actions/workflows/codeql.yml
<!-- Badges: release & docker images  -->
[release-badge]: https://badge.fury.io/gh/go-openapi%2Fstrfmt.svg
[release-url]: https://badge.fury.io/gh/go-openapi%2Fstrfmt
[gomod-badge]: https://badge.fury.io/go/github.com%2Fgo-openapi%2Fstrfmt.svg
[gomod-url]: https://badge.fury.io/go/github.com%2Fgo-openapi%2Fstrfmt
<!-- Badges: code quality  -->
[gocard-badge]: https://goreportcard.com/badge/github.com/go-openapi/strfmt
[gocard-url]: https://goreportcard.com/report/github.com/go-openapi/strfmt
[codefactor-badge]: https://img.shields.io/codefactor/grade/github/go-openapi/strfmt
[codefactor-url]: https://www.codefactor.io/repository/github/go-openapi/strfmt
<!-- Badges: documentation & support -->
[doc-badge]: https://img.shields.io/badge/doc-site-blue?link=https%3A%2F%2Fgoswagger.io%2Fgo-openapi%2F
[doc-url]: https://goswagger.io/go-openapi
[godoc-badge]: https://pkg.go.dev/badge/github.com/go-openapi/strfmt
[godoc-url]: http://pkg.go.dev/github.com/go-openapi/strfmt
[discord-badge]: https://img.shields.io/discord/1446918742398341256?logo=discord&label=discord&color=blue
[discord-url]: https://discord.gg/FfnFYaC3k5

<!-- Badges: license & compliance -->
[license-badge]: http://img.shields.io/badge/license-Apache%20v2-orange.svg
[license-url]: https://github.com/go-openapi/strfmt/?tab=Apache-2.0-1-ov-file#readme
<!-- Badges: others & stats -->
[goversion-badge]: https://img.shields.io/github/go-mod/go-version/go-openapi/strfmt
[goversion-url]: https://github.com/go-openapi/strfmt/blob/master/go.mod
[top-badge]: https://img.shields.io/github/languages/top/go-openapi/strfmt
[commits-badge]: https://img.shields.io/github/commits-since/go-openapi/strfmt/latest
<!-- Organization docs -->
[contributing-doc-site]: https://go-openapi.github.io/doc-site/contributing/contributing/index.html
[maintainers-doc-site]: https://go-openapi.github.io/doc-site/maintainers/index.html
[style-doc-site]: https://go-openapi.github.io/doc-site/contributing/style/index.html
