# Release (2022-12-02)

* No change notes available for this release.

# Release (2022-10-24)

## Module Highlights
* `github.com/aws/smithy-go`: v1.13.4
  * **Bug Fix**: fixed document type checking for encoding nested types

# Release (2022-09-14)

* No change notes available for this release.

# Release (v1.13.2)

* No change notes available for this release.

# Release (v1.13.1)

* No change notes available for this release.

# Release (v1.13.0)

## Module Highlights
* `github.com/aws/smithy-go`: v1.13.0
  * **Feature**: Adds support for the Smithy httpBearerAuth authentication trait to smithy-go. This allows the SDK to support the bearer authentication flow for API operations decorated with httpBearerAuth. An API client will need to be provided with its own bearer.TokenProvider implementation or use the bearer.StaticTokenProvider implementation.

# Release (v1.12.1)

## Module Highlights
* `github.com/aws/smithy-go`: v1.12.1
  * **Bug Fix**: Fixes a bug where JSON object keys were not escaped.

# Release (v1.12.0)

## Module Highlights
* `github.com/aws/smithy-go`: v1.12.0
  * **Feature**: `transport/http`: Add utility for setting context metadata when operation serializer automatically assigns content-type default value.

# Release (v1.11.3)

## Module Highlights
* `github.com/aws/smithy-go`: v1.11.3
  * **Dependency Update**: Updates smithy-go unit test dependency go-cmp to 0.5.8.

# Release (v1.11.2)

* No change notes available for this release.

# Release (v1.11.1)

## Module Highlights
* `github.com/aws/smithy-go`: v1.11.1
  * **Bug Fix**: Updates the smithy-go HTTP Request to correctly handle building the request to an http.Request. Related to [aws/aws-sdk-go-v2#1583](https://github.com/aws/aws-sdk-go-v2/issues/1583)

# Release (v1.11.0)

## Module Highlights
* `github.com/aws/smithy-go`: v1.11.0
  * **Feature**: Updates deserialization of header list to supported quoted strings

# Release (v1.10.0)

## Module Highlights
* `github.com/aws/smithy-go`: v1.10.0
  * **Feature**: Add `ptr.Duration`, `ptr.ToDuration`, `ptr.DurationSlice`, `ptr.ToDurationSlice`, `ptr.DurationMap`, and `ptr.ToDurationMap` functions for the `time.Duration` type.

# Release (v1.9.1)

## Module Highlights
* `github.com/aws/smithy-go`: v1.9.1
  * **Documentation**: Fixes various typos in Go package documentation.

# Release (v1.9.0)

## Module Highlights
* `github.com/aws/smithy-go`: v1.9.0
  * **Feature**: sync: OnceErr, can be used to concurrently record a signal when an error has occurred.
  * **Bug Fix**: `transport/http`: CloseResponseBody and ErrorCloseResponseBody middleware have been updated to ensure that the body is fully drained before closing.

# Release v1.8.1

### Smithy Go Module
* **Bug Fix**: Fixed an issue that would cause the HTTP Content-Length to be set to 0 if the stream body was not set.
  * Fixes [aws/aws-sdk-go-v2#1418](https://github.com/aws/aws-sdk-go-v2/issues/1418)

# Release v1.8.0

### Smithy Go Module

* `time`: Add support for parsing additional DateTime timestamp format ([#324](https://github.com/aws/smithy-go/pull/324))
  * Adds support for parsing DateTime timestamp formatted time similar to RFC 3339, but without the `Z` character, nor UTC offset.
  * Fixes [#1387](https://github.com/aws/aws-sdk-go-v2/issues/1387)

# Release v1.7.0

### Smithy Go Module
* `ptr`:  Handle error for deferred file close call ([#314](https://github.com/aws/smithy-go/pull/314))
  * Handle error for defer close call
* `middleware`: Add Clone to Metadata ([#318](https://github.com/aws/smithy-go/pull/318))
  * Adds a new Clone method to the middleware Metadata type. This provides a shallow clone of the entries in the Metadata.
* `document`: Add new package for document shape serialization support ([#310](https://github.com/aws/smithy-go/pull/310))

### Codegen
* Add Smithy Document Shape Support ([#310](https://github.com/aws/smithy-go/pull/310))
  * Adds support for Smithy Document shapes and supporting types for protocols to implement support

# Release v1.6.0 (2021-07-15)

### Smithy Go Module
* `encoding/httpbinding`: Support has been added for encoding `float32` and `float64` values that are `NaN`, `Infinity`, or `-Infinity`. ([#316](https://github.com/aws/smithy-go/pull/316))

### Codegen
* Adds support for handling `float32` and `float64` `NaN` values in HTTP Protocol Unit Tests. ([#316](https://github.com/aws/smithy-go/pull/316))
* Adds support protocol generator implementations to override the error code string returned by `ErrorCode` methods on generated error types. ([#315](https://github.com/aws/smithy-go/pull/315))

# Release v1.5.0 (2021-06-25)

### Smithy Go module
* `time`: Update time parsing to not be as strict for HTTPDate and DateTime ([#307](https://github.com/aws/smithy-go/pull/307))
  * Fixes [#302](https://github.com/aws/smithy-go/issues/302) by changing time to UTC before formatting so no local offset time is lost.

### Codegen
* Adds support for integrating client members via plugins ([#301](https://github.com/aws/smithy-go/pull/301))
* Fix serialization of enum types marked with payload trait ([#296](https://github.com/aws/smithy-go/pull/296))
* Update generation of API client modules to include a manifest of files generated ([#283](https://github.com/aws/smithy-go/pull/283))
* Update Group Java group ID for smithy-go generator ([#298](https://github.com/aws/smithy-go/pull/298))
* Support the delegation of determining the errors that can occur for an operation ([#304](https://github.com/aws/smithy-go/pull/304))
* Support for marking and documenting deprecated client config fields. ([#303](https://github.com/aws/smithy-go/pull/303))

# Release v1.4.0 (2021-05-06)

### Smithy Go module
* `encoding/xml`: Fix escaping of Next Line and Line Start in XML Encoder ([#267](https://github.com/aws/smithy-go/pull/267))

### Codegen
* Add support for Smithy 1.7 ([#289](https://github.com/aws/smithy-go/pull/289))
* Add support for httpQueryParams location
* Add support for model renaming conflict resolution with service closure

# Release v1.3.1 (2021-04-08)

### Smithy Go module
* `transport/http`: Loosen endpoint hostname validation to allow specifying port numbers. ([#279](https://github.com/aws/smithy-go/pull/279))
* `io`: Fix RingBuffer panics due to out of bounds index. ([#282](https://github.com/aws/smithy-go/pull/282))

# Release v1.3.0 (2021-04-01)

### Smithy Go module
* `transport/http`: Add utility to safely join string to url path, and url raw query.

### Codegen
* Update HttpBindingProtocolGenerator to use http/transport JoinPath and JoinQuery utility.

# Release v1.2.0 (2021-03-12)

### Smithy Go module
* Fix support for parsing shortened year format in HTTP Date header.
* Fix GitHub APIDiff action workflow to get gorelease tool correctly.
* Fix codegen artifact unit test for Go 1.16

### Codegen
* Fix generating paginator nil parameter handling before usage.
* Fix Serialize unboxed members decorated as required.
* Add ability to define resolvers at both client construction and operation invocation.
* Support for extending paginators with custom runtime trait
