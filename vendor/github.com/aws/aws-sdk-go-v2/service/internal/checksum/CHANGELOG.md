# v1.1.5 (2022-04-27)

* **Bug Fix**: Fixes a bug that could cause the SigV4 payload hash to be incorrectly encoded, leading to signing errors.

# v1.1.4 (2022-04-25)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.1.3 (2022-03-30)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.1.2 (2022-03-24)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.1.1 (2022-03-23)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.1.0 (2022-03-08)

* **Feature**:  Updates the SDK's checksum validation logic to require opt-in to output response payload validation. The SDK was always preforming output response payload checksum validation, not respecting the output validation model option. Fixes [#1606](https://github.com/aws/aws-sdk-go-v2/issues/1606)
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.0.0 (2022-02-24)

* **Release**: New module for computing checksums
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

