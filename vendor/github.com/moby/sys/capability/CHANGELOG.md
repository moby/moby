# Changelog
This file documents all notable changes made to this project since the initial fork
from https://github.com/syndtr/gocapability/commit/42c35b4376354fd5.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2024-11-11

### Added
* New separate API for ambient ([GetAmbient], [SetAmbient], [ResetAmbient])
  and bound ([GetBound], [DropBound]) capabilities, modelled after libcap. (#176)

### Fixed
* [Apply] now returns an error if called for non-zero `pid`. Before this change,
  it could silently change some capabilities of the current process, instead of
  the one identified by the `pid`. (#168, #174)
* Fixed tests that change capabilities to be run in a separate process. (#173)
* Other improvements in tests. (#169, #170)

### Changed
* Use raw syscalls (which are slightly faster). (#176)
* Most tests are now limited to testing the public API of the package. (#162)
* Simplify parsing /proc/*pid*/status, add a test case. (#162)
* Optimize the number of syscall to set ambient capabilities in Apply
  by clearing them first; add a test case. (#163, #164)
* Better documentation for [Apply], [NewFile], [NewFile2], [NewPid], [NewPid2]. (#175)

### Removed
* `.golangci.yml` and `.codespellrc` are no longer part of the package. (#158)

## [0.3.0] - 2024-09-25

### Added
* Added [ListKnown] and [ListSupported] functions. (#153)
* [LastCap] is now available on non-Linux platforms (where it returns an error). (#152)

### Changed
* [List] is now deprecated in favor of [ListKnown] and [ListSupported]. (#153)

### Fixed
* Various documentation improvements. (#151)
* Fix "generated code" comment. (#153)

## [0.2.0] - 2024-09-16

This is the first release after the move to a new home in
github.com/moby/sys/capability.

### Fixed
 * Fixed URLs in documentation to reflect the new home.

## [0.1.1] - 2024-08-01

This is a maintenance release, fixing a few minor issues.

### Fixed
 * Fixed future kernel compatibility, for real this time. [#11]
 * Fixed [LastCap] to be a function. [#12]

## [0.1.0] - 2024-07-31

This is an initial release since the fork.

### Breaking changes

 * The `CAP_LAST_CAP` variable is removed; users need to modify the code to
   use [LastCap] to get the value. [#6]
 * The code now requires Go >= 1.21.

### Added
 * `go.mod` and `go.sum` files. [#2]
 * New [LastCap] function. [#6]
 * Basic CI using GHA infra. [#8], [#9]
 * README and CHANGELOG. [#10]

### Fixed
 * Fixed ambient capabilities error handling in [Apply]. [#3]
 * Fixed future kernel compatibility. [#1]
 * Fixed various linter warnings. [#4], [#7]

### Changed
 * Go build tags changed from old-style (`+build`) to new Go 1.17+ style (`go:build`). [#2]

### Removed
 * Removed support for capabilities v1 and v2. [#1]
 * Removed init function so programs that use this package start faster. [#6]
 * Removed `CAP_LAST_CAP` (use [LastCap] instead). [#6]

<!-- Doc links (please keep sorted). -->
[Apply]: https://pkg.go.dev/github.com/moby/sys/capability#Capabilities.Apply
[DropBound]: https://pkg.go.dev/github.com/moby/sys/capability#DropBound
[GetAmbient]: https://pkg.go.dev/github.com/moby/sys/capability#GetAmbient
[GetBound]: https://pkg.go.dev/github.com/moby/sys/capability#GetBound
[LastCap]: https://pkg.go.dev/github.com/moby/sys/capability#LastCap
[ListKnown]: https://pkg.go.dev/github.com/moby/sys/capability#ListKnown
[ListSupported]: https://pkg.go.dev/github.com/moby/sys/capability#ListSupported
[List]: https://pkg.go.dev/github.com/moby/sys/capability#List
[NewFile2]: https://pkg.go.dev/github.com/moby/sys/capability#NewFile2
[NewFile]: https://pkg.go.dev/github.com/moby/sys/capability#NewFile
[NewPid2]: https://pkg.go.dev/github.com/moby/sys/capability#NewPid2
[NewPid]: https://pkg.go.dev/github.com/moby/sys/capability#NewPid
[ResetAmbient]: https://pkg.go.dev/github.com/moby/sys/capability#ResetAmbient
[SetAmbient]: https://pkg.go.dev/github.com/moby/sys/capability#SetAmbient

<!-- Minor releases. -->
[0.4.0]: https://github.com/moby/sys/releases/tag/capability%2Fv0.4.0
[0.3.0]: https://github.com/moby/sys/releases/tag/capability%2Fv0.3.0
[0.2.0]: https://github.com/moby/sys/releases/tag/capability%2Fv0.2.0
[0.1.1]: https://github.com/kolyshkin/capability/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/kolyshkin/capability/compare/42c35b4376354fd5...v0.1.0

<!-- PRs in 0.1.x releases. -->
[#1]: https://github.com/kolyshkin/capability/pull/1
[#2]: https://github.com/kolyshkin/capability/pull/2
[#3]: https://github.com/kolyshkin/capability/pull/3
[#4]: https://github.com/kolyshkin/capability/pull/4
[#6]: https://github.com/kolyshkin/capability/pull/6
[#7]: https://github.com/kolyshkin/capability/pull/7
[#8]: https://github.com/kolyshkin/capability/pull/8
[#9]: https://github.com/kolyshkin/capability/pull/9
[#10]: https://github.com/kolyshkin/capability/pull/10
[#11]: https://github.com/kolyshkin/capability/pull/11
[#12]: https://github.com/kolyshkin/capability/pull/12
