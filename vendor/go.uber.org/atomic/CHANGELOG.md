# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.9.0] - 2021-07-15
### Added
- Add `Float64.Swap` to match int atomic operations.
- Add `atomic.Time` type for atomic operations on `time.Time` values.

[1.9.0]: https://github.com/uber-go/atomic/compare/v1.8.0...v1.9.0

## [1.8.0] - 2021-06-09
### Added
- Add `atomic.Uintptr` type for atomic operations on `uintptr` values.
- Add `atomic.UnsafePointer` type for atomic operations on `unsafe.Pointer` values.

[1.8.0]: https://github.com/uber-go/atomic/compare/v1.7.0...v1.8.0

## [1.7.0] - 2020-09-14
### Added
- Support JSON serialization and deserialization of primitive atomic types.
- Support Text marshalling and unmarshalling for string atomics.

### Changed
- Disallow incorrect comparison of atomic values in a non-atomic way.

### Removed
- Remove dependency on `golang.org/x/{lint, tools}`.

[1.7.0]: https://github.com/uber-go/atomic/compare/v1.6.0...v1.7.0

## [1.6.0] - 2020-02-24
### Changed
- Drop library dependency on `golang.org/x/{lint, tools}`.

[1.6.0]: https://github.com/uber-go/atomic/compare/v1.5.1...v1.6.0

## [1.5.1] - 2019-11-19
- Fix bug where `Bool.CAS` and `Bool.Toggle` do work correctly together
  causing `CAS` to fail even though the old value matches.

[1.5.1]: https://github.com/uber-go/atomic/compare/v1.5.0...v1.5.1

## [1.5.0] - 2019-10-29
### Changed
- With Go modules, only the `go.uber.org/atomic` import path is supported now.
  If you need to use the old import path, please add a `replace` directive to
  your `go.mod`.

[1.5.0]: https://github.com/uber-go/atomic/compare/v1.4.0...v1.5.0

## [1.4.0] - 2019-05-01
### Added
 - Add `atomic.Error` type for atomic operations on `error` values.

[1.4.0]: https://github.com/uber-go/atomic/compare/v1.3.2...v1.4.0

## [1.3.2] - 2018-05-02
### Added
- Add `atomic.Duration` type for atomic operations on `time.Duration` values.

[1.3.2]: https://github.com/uber-go/atomic/compare/v1.3.1...v1.3.2

## [1.3.1] - 2017-11-14
### Fixed
- Revert optimization for `atomic.String.Store("")` which caused data races.

[1.3.1]: https://github.com/uber-go/atomic/compare/v1.3.0...v1.3.1

## [1.3.0] - 2017-11-13
### Added
- Add `atomic.Bool.CAS` for compare-and-swap semantics on bools.

### Changed
- Optimize `atomic.String.Store("")` by avoiding an allocation.

[1.3.0]: https://github.com/uber-go/atomic/compare/v1.2.0...v1.3.0

## [1.2.0] - 2017-04-12
### Added
- Shadow `atomic.Value` from `sync/atomic`.

[1.2.0]: https://github.com/uber-go/atomic/compare/v1.1.0...v1.2.0

## [1.1.0] - 2017-03-10
### Added
- Add atomic `Float64` type.

### Changed
- Support new `go.uber.org/atomic` import path.

[1.1.0]: https://github.com/uber-go/atomic/compare/v1.0.0...v1.1.0

## [1.0.0] - 2016-07-18

- Initial release.

[1.0.0]: https://github.com/uber-go/atomic/releases/tag/v1.0.0
