# v1.26.9 (2022-05-06)

* No change notes available for this release.

# v1.26.8 (2022-05-03)

* **Documentation**: Documentation only update for doc bug fixes for the S3 API docs.

# v1.26.7 (2022-04-27)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.26.6 (2022-04-25)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.26.5 (2022-04-12)

* **Bug Fix**: Fixes an issue that caused the unexported constructor function names for EventStream types to be swapped for the event reader and writer respectivly.

# v1.26.4 (2022-04-07)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.26.3 (2022-03-30)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.26.2 (2022-03-24)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.26.1 (2022-03-23)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.26.0 (2022-03-08)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.25.0 (2022-02-24)

* **Feature**: API client updated
* **Feature**: Adds RetryMaxAttempts and RetryMod to API client Options. This allows the API clients' default Retryer to be configured from the shared configuration files or environment variables. Adding a new Retry mode of `Adaptive`. `Adaptive` retry mode is an experimental mode, adding client rate limiting when throttles reponses are received from an API. See [retry.AdaptiveMode](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/retry#AdaptiveMode) for more details, and configuration options.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Bug Fix**: Fixes the AWS Sigv4 signer to trim header value's whitespace when computing the canonical headers block of the string to sign.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.24.1 (2022-01-28)

* **Bug Fix**: Updates SDK API client deserialization to pre-allocate byte slice and string response payloads, [#1565](https://github.com/aws/aws-sdk-go-v2/pull/1565). Thanks to [Tyson Mote](https://github.com/tysonmote) for submitting this PR.

# v1.24.0 (2022-01-14)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.23.0 (2022-01-07)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Documentation**: API client updated
* **Dependency Update**: Updated to the latest SDK module versions

# v1.22.0 (2021-12-21)

* **Feature**: API Paginators now support specifying the initial starting token, and support stopping on empty string tokens.
* **Feature**: Updated to latest service endpoints

# v1.21.0 (2021-12-02)

* **Feature**: API client updated
* **Bug Fix**: Fixes a bug that prevented aws.EndpointResolverWithOptions from being used by the service client. ([#1514](https://github.com/aws/aws-sdk-go-v2/pull/1514))
* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.0 (2021-11-30)

* **Feature**: API client updated

# v1.19.1 (2021-11-19)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.19.0 (2021-11-12)

* **Feature**: Waiters now have a `WaitForOutput` method, which can be used to retrieve the output of the successful wait operation. Thank you to [Andrew Haines](https://github.com/haines) for contributing this feature.

# v1.18.0 (2021-11-06)

* **Feature**: Support has been added for the SelectObjectContent API.
* **Feature**: The SDK now supports configuration of FIPS and DualStack endpoints using environment variables, shared configuration, or programmatically.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Feature**: Updated service to latest API model.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.0 (2021-10-21)

* **Feature**: Updated  to latest version
* **Feature**: Updates S3 streaming operations - PutObject, UploadPart, WriteGetObjectResponse to use unsigned payload signing auth when TLS is enabled.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.16.1 (2021-10-11)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.16.0 (2021-09-17)

* **Feature**: Updated API client and endpoints to latest revision.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.1 (2021-09-10)

* No change notes available for this release.

# v1.15.0 (2021-09-02)

* **Feature**: API client updated
* **Feature**: Add support for S3 Multi-Region Access Point ARNs.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.14.0 (2021-08-27)

* **Feature**: Updated API model to latest revision.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.13.0 (2021-08-19)

* **Feature**: API client updated
* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.0 (2021-08-04)

* **Feature**: Add `HeadObject` presign support. ([#1346](https://github.com/aws/aws-sdk-go-v2/pull/1346))
* **Dependency Update**: Updated `github.com/aws/smithy-go` to latest version.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.1 (2021-07-15)

* **Dependency Update**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.0 (2021-06-25)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.10.0 (2021-06-04)

* **Feature**: The handling of AccessPoint and Outpost ARNs have been updated.
* **Feature**: Updated service client to latest API model.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.9.0 (2021-05-25)

* **Feature**: API client updated

# v1.8.0 (2021-05-20)

* **Feature**: API client updated
* **Dependency Update**: Updated to the latest SDK module versions

# v1.7.0 (2021-05-14)

* **Feature**: Constant has been added to modules to enable runtime version inspection for reporting.
* **Feature**: Updated to latest service API model.
* **Dependency Update**: Updated to the latest SDK module versions

