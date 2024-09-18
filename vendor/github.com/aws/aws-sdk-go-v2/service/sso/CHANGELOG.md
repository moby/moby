# v1.18.7 (2024-01-18)

* No change notes available for this release.

# v1.18.6 (2024-01-04)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.5 (2023-12-08)

* **Bug Fix**: Reinstate presence of default Retryer in functional options, but still respect max attempts set therein.

# v1.18.4 (2023-12-07)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.3 (2023-12-06)

* **Bug Fix**: Restore pre-refactor auth behavior where all operations could technically be performed anonymously.

# v1.18.2 (2023-12-01)

* **Bug Fix**: Correct wrapping of errors in authentication workflow.
* **Bug Fix**: Correctly recognize cache-wrapped instances of AnonymousCredentials at client construction.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.1 (2023-11-30)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.0 (2023-11-29)

* **Feature**: Expose Options() accessor on service clients.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.5 (2023-11-28.2)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.4 (2023-11-28)

* **Bug Fix**: Respect setting RetryMaxAttempts in functional options at client construction.

# v1.17.3 (2023-11-20)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.2 (2023-11-15)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.1 (2023-11-09)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.0 (2023-11-01)

* **Feature**: Adds support for configured endpoints via environment variables and the AWS shared configuration file.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.16.0 (2023-10-31)

* **Feature**: **BREAKING CHANGE**: Bump minimum go version to 1.19 per the revised [go version support policy](https://aws.amazon.com/blogs/developer/aws-sdk-for-go-aligns-with-go-release-policy-on-supported-runtimes/).
* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.2 (2023-10-12)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.1 (2023-10-06)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.0 (2023-10-02)

* **Feature**: Fix FIPS Endpoints in aws-us-gov.

# v1.14.1 (2023-09-22)

* No change notes available for this release.

# v1.14.0 (2023-09-18)

* **Announcement**: [BREAKFIX] Change in MaxResults datatype from value to pointer type in cognito-sync service.
* **Feature**: Adds several endpoint ruleset changes across all models: smaller rulesets, removed non-unique regional endpoints, fixes FIPS and DualStack endpoints, and make region not required in SDK::Endpoint. Additional breakfix to cognito-sync field.

# v1.13.6 (2023-08-31)

* No change notes available for this release.

# v1.13.5 (2023-08-21)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.13.4 (2023-08-18)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.13.3 (2023-08-17)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.13.2 (2023-08-07)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.13.1 (2023-08-01)

* No change notes available for this release.

# v1.13.0 (2023-07-31)

* **Feature**: Adds support for smithy-modeled endpoint resolution. A new rules-based endpoint resolution will be added to the SDK which will supercede and deprecate existing endpoint resolution. Specifically, EndpointResolver will be deprecated while BaseEndpoint and EndpointResolverV2 will take its place. For more information, please see the Endpoints section in our Developer Guide.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.14 (2023-07-28)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.13 (2023-07-13)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.12 (2023-06-15)

* No change notes available for this release.

# v1.12.11 (2023-06-13)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.10 (2023-05-04)

* No change notes available for this release.

# v1.12.9 (2023-04-24)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.8 (2023-04-10)

* No change notes available for this release.

# v1.12.7 (2023-04-07)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.6 (2023-03-21)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.5 (2023-03-10)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.4 (2023-02-22)

* **Bug Fix**: Prevent nil pointer dereference when retrieving error codes.

# v1.12.3 (2023-02-20)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.2 (2023-02-15)

* **Announcement**: When receiving an error response in restJson-based services, an incorrect error type may have been returned based on the content of the response. This has been fixed via PR #2012 tracked in issue #1910.
* **Bug Fix**: Correct error type parsing for restJson services.

# v1.12.1 (2023-02-03)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.0 (2023-01-05)

* **Feature**: Add `ErrorCodeOverride` field to all error structs (aws/smithy-go#401).

# v1.11.28 (2022-12-20)

* No change notes available for this release.

# v1.11.27 (2022-12-15)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.26 (2022-12-02)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.25 (2022-10-24)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.24 (2022-10-21)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.23 (2022-09-20)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.22 (2022-09-14)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.21 (2022-09-02)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.20 (2022-08-31)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.19 (2022-08-30)

* **Documentation**: Documentation updates for the AWS IAM Identity Center Portal CLI Reference.

# v1.11.18 (2022-08-29)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.17 (2022-08-15)

* **Documentation**: Documentation updates to reflect service rename - AWS IAM Identity Center (successor to AWS Single Sign-On)

# v1.11.16 (2022-08-11)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.15 (2022-08-09)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.14 (2022-08-08)

* **Documentation**: Documentation updates to reflect service rename - AWS IAM Identity Center (successor to AWS Single Sign-On)
* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.13 (2022-08-01)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.12 (2022-07-11)

* No change notes available for this release.

# v1.11.11 (2022-07-05)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.10 (2022-06-29)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.9 (2022-06-16)

* No change notes available for this release.

# v1.11.8 (2022-06-07)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.7 (2022-05-26)

* No change notes available for this release.

# v1.11.6 (2022-05-25)

* No change notes available for this release.

# v1.11.5 (2022-05-17)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.4 (2022-04-25)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.3 (2022-03-30)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.2 (2022-03-24)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.1 (2022-03-23)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.0 (2022-03-08)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.10.0 (2022-02-24)

* **Feature**: API client updated
* **Feature**: Adds RetryMaxAttempts and RetryMod to API client Options. This allows the API clients' default Retryer to be configured from the shared configuration files or environment variables. Adding a new Retry mode of `Adaptive`. `Adaptive` retry mode is an experimental mode, adding client rate limiting when throttles reponses are received from an API. See [retry.AdaptiveMode](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/retry#AdaptiveMode) for more details, and configuration options.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.9.0 (2022-01-14)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Documentation**: Updated API models
* **Dependency Update**: Updated to the latest SDK module versions

# v1.8.0 (2022-01-07)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.7.0 (2021-12-21)

* **Feature**: API Paginators now support specifying the initial starting token, and support stopping on empty string tokens.

# v1.6.2 (2021-12-02)

* **Bug Fix**: Fixes a bug that prevented aws.EndpointResolverWithOptions from being used by the service client. ([#1514](https://github.com/aws/aws-sdk-go-v2/pull/1514))
* **Dependency Update**: Updated to the latest SDK module versions

# v1.6.1 (2021-11-19)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.6.0 (2021-11-06)

* **Feature**: The SDK now supports configuration of FIPS and DualStack endpoints using environment variables, shared configuration, or programmatically.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Feature**: Updated service to latest API model.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.5.0 (2021-10-21)

* **Feature**: Updated  to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.4.2 (2021-10-11)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.4.1 (2021-09-17)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.4.0 (2021-08-27)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.3.3 (2021-08-19)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.3.2 (2021-08-04)

* **Dependency Update**: Updated `github.com/aws/smithy-go` to latest version.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.3.1 (2021-07-15)

* **Dependency Update**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.3.0 (2021-06-25)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.2.1 (2021-05-20)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.2.0 (2021-05-14)

* **Feature**: Constant has been added to modules to enable runtime version inspection for reporting.
* **Dependency Update**: Updated to the latest SDK module versions

