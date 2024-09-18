# v1.32.0 (2024-02-06)

* **Feature**: This release adds a new field, logGroupArn, to the response of the logs:DescribeLogGroups action.

# v1.31.0 (2024-01-10)

* **Feature**: Add support for account level subscription filter policies to PutAccountPolicy, DescribeAccountPolicies, and DeleteAccountPolicy APIs. Additionally, PutAccountPolicy has been modified with new optional "selectionCriteria" parameter for resource selection.

# v1.30.2 (2024-01-04)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.30.1 (2023-12-20)

* No change notes available for this release.

# v1.30.0 (2023-12-12)

* **Feature**: This release introduces the StartLiveTail API to tail ingested logs in near real time.

# v1.29.5 (2023-12-08)

* **Bug Fix**: Reinstate presence of default Retryer in functional options, but still respect max attempts set therein.

# v1.29.4 (2023-12-07)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.29.3 (2023-12-06)

* **Bug Fix**: Restore pre-refactor auth behavior where all operations could technically be performed anonymously.

# v1.29.2 (2023-12-01)

* **Bug Fix**: Correct wrapping of errors in authentication workflow.
* **Bug Fix**: Correctly recognize cache-wrapped instances of AnonymousCredentials at client construction.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.29.1 (2023-11-30)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.29.0 (2023-11-29)

* **Feature**: Expose Options() accessor on service clients.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.28.2 (2023-11-28.2)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.28.1 (2023-11-28)

* **Bug Fix**: Respect setting RetryMaxAttempts in functional options at client construction.

# v1.28.0 (2023-11-27)

* **Feature**: Added APIs to Create, Update, Get, List and Delete LogAnomalyDetectors and List and Update Anomalies in Detector. Added LogGroupClass attribute for LogGroups to classify loggroup as Standard loggroup with all capabilities or InfrequentAccess loggroup with limited capabilities.

# v1.27.2 (2023-11-20)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.27.1 (2023-11-15)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.27.0 (2023-11-09.2)

* **Feature**: Update to support new APIs for delivery of logs from AWS services.

# v1.26.1 (2023-11-09)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.26.0 (2023-11-01)

* **Feature**: Adds support for configured endpoints via environment variables and the AWS shared configuration file.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.25.0 (2023-10-31)

* **Feature**: **BREAKING CHANGE**: Bump minimum go version to 1.19 per the revised [go version support policy](https://aws.amazon.com/blogs/developer/aws-sdk-for-go-aligns-with-go-release-policy-on-supported-runtimes/).
* **Dependency Update**: Updated to the latest SDK module versions

# v1.24.2 (2023-10-12)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.24.1 (2023-10-06)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.24.0 (2023-09-20)

* **Feature**: Add ClientToken to QueryDefinition CFN Handler in CWL

# v1.23.5 (2023-08-21)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.23.4 (2023-08-18)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.23.3 (2023-08-17)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.23.2 (2023-08-07)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.23.1 (2023-08-01)

* No change notes available for this release.

# v1.23.0 (2023-07-31)

* **Feature**: Adds support for smithy-modeled endpoint resolution. A new rules-based endpoint resolution will be added to the SDK which will supercede and deprecate existing endpoint resolution. Specifically, EndpointResolver will be deprecated while BaseEndpoint and EndpointResolverV2 will take its place. For more information, please see the Endpoints section in our Developer Guide.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.22.2 (2023-07-28)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.22.1 (2023-07-13)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.22.0 (2023-07-07)

* **Feature**: Add CMK encryption support for CloudWatch Logs Insights query result data

# v1.21.2 (2023-06-15)

* No change notes available for this release.

# v1.21.1 (2023-06-13)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.21.0 (2023-06-07)

* **Feature**: This change adds support for account level data protection policies using 3 new APIs, PutAccountPolicy, DeleteAccountPolicy and DescribeAccountPolicy. DescribeLogGroup API has been modified to indicate if account level policy is applied to the LogGroup via "inheritedProperties" list in the response.

# v1.20.11 (2023-05-04)

* No change notes available for this release.

# v1.20.10 (2023-04-24)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.9 (2023-04-10)

* No change notes available for this release.

# v1.20.8 (2023-04-07)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.7 (2023-03-21)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.6 (2023-03-10)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.5 (2023-02-22)

* **Bug Fix**: Prevent nil pointer dereference when retrieving error codes.

# v1.20.4 (2023-02-20)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.3 (2023-02-15)

* **Announcement**: When receiving an error response in restJson-based services, an incorrect error type may have been returned based on the content of the response. This has been fixed via PR #2012 tracked in issue #1910.
* **Bug Fix**: Correct error type parsing for restJson services.

# v1.20.2 (2023-02-03)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.1 (2023-01-23)

* No change notes available for this release.

# v1.20.0 (2023-01-19)

* **Feature**: Bug fix - Removed the regex pattern validation from CoralModel to avoid potential security issue.

# v1.19.0 (2023-01-12)

* **Feature**: Bug fix: logGroupName is now not a required field in GetLogEvents, FilterLogEvents, GetLogGroupFields, and DescribeLogStreams APIs as logGroupIdentifier can be provided instead

# v1.18.0 (2023-01-05)

* **Feature**: Add `ErrorCodeOverride` field to all error structs (aws/smithy-go#401).

# v1.17.4 (2023-01-04)

* **Documentation**: Update to remove sequenceToken as a required field in PutLogEvents calls.

# v1.17.3 (2022-12-15)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.2 (2022-12-09)

* **Documentation**: Doc-only update for CloudWatch Logs, for Tagging Permissions clarifications

# v1.17.1 (2022-12-02)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.0 (2022-11-28)

* **Feature**: Updates to support CloudWatch Logs data protection and CloudWatch cross-account observability

# v1.16.4 (2022-11-22)

* No change notes available for this release.

# v1.16.3 (2022-11-16)

* No change notes available for this release.

# v1.16.2 (2022-11-10)

* No change notes available for this release.

# v1.16.1 (2022-11-04)

* **Documentation**: Doc-only update for bug fixes and support of export to buckets encrypted with SSE-KMS

# v1.16.0 (2022-10-31)

* **Feature**: SDK release to support tagging for destinations and log groups with TagResource. Also supports tag on create with PutDestination.

# v1.15.22 (2022-10-24)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.21 (2022-10-21)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.20 (2022-09-20)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.19 (2022-09-14)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.18 (2022-09-02)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.17 (2022-08-31)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.16 (2022-08-30)

* No change notes available for this release.

# v1.15.15 (2022-08-29)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.14 (2022-08-11)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.13 (2022-08-09)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.12 (2022-08-08)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.11 (2022-08-01)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.10 (2022-07-05)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.9 (2022-06-29)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.8 (2022-06-07)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.7 (2022-05-20)

* **Documentation**: Doc-only update to publish the new valid values for log retention

# v1.15.6 (2022-05-17)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.5 (2022-04-25)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.4 (2022-03-30)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.3 (2022-03-24)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.2 (2022-03-23)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.1 (2022-03-08.2)

* No change notes available for this release.

# v1.15.0 (2022-03-08)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.14.0 (2022-02-24)

* **Feature**: API client updated
* **Feature**: Adds RetryMaxAttempts and RetryMod to API client Options. This allows the API clients' default Retryer to be configured from the shared configuration files or environment variables. Adding a new Retry mode of `Adaptive`. `Adaptive` retry mode is an experimental mode, adding client rate limiting when throttles reponses are received from an API. See [retry.AdaptiveMode](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/retry#AdaptiveMode) for more details, and configuration options.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.13.0 (2022-01-14)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.0 (2022-01-07)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.0 (2021-12-21)

* **Feature**: API Paginators now support specifying the initial starting token, and support stopping on empty string tokens.
* **Feature**: API client updated
* **Feature**: Updated to latest service endpoints

# v1.10.2 (2021-12-02)

* **Bug Fix**: Fixes a bug that prevented aws.EndpointResolverWithOptions from being used by the service client. ([#1514](https://github.com/aws/aws-sdk-go-v2/pull/1514))
* **Dependency Update**: Updated to the latest SDK module versions

# v1.10.1 (2021-11-19)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.10.0 (2021-11-12)

* **Feature**: Service clients now support custom endpoints that have an initial URI path defined.

# v1.9.0 (2021-11-06)

* **Feature**: The SDK now supports configuration of FIPS and DualStack endpoints using environment variables, shared configuration, or programmatically.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.8.0 (2021-10-21)

* **Feature**: API client updated
* **Feature**: Updated  to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.7.2 (2021-10-11)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.7.1 (2021-09-17)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.7.0 (2021-08-27)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.6.0 (2021-08-19)

* **Feature**: API client updated
* **Dependency Update**: Updated to the latest SDK module versions

# v1.5.2 (2021-08-04)

* **Dependency Update**: Updated `github.com/aws/smithy-go` to latest version.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.5.1 (2021-07-15)

* **Dependency Update**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.5.0 (2021-06-25)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.4.0 (2021-05-25)

* **Feature**: API client updated

# v1.3.1 (2021-05-20)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.3.0 (2021-05-14)

* **Feature**: Constant has been added to modules to enable runtime version inspection for reporting.
* **Dependency Update**: Updated to the latest SDK module versions

