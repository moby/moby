# v1.17.4 (2022-08-31)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.3 (2022-08-30)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.2 (2022-08-29)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.1 (2022-08-15)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.0 (2022-08-14)

* **Feature**: Add alternative mechanism for determning the users `$HOME` or `%USERPROFILE%` location when the environment variables are not present.

# v1.16.1 (2022-08-11)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.16.0 (2022-08-10)

* **Feature**: Adds support for the following settings in the `~/.aws/credentials` file: `sso_account_id`, `sso_region`, `sso_role_name`, `sso_start_url`, and `ca_bundle`.

# v1.15.17 (2022-08-09)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.16 (2022-08-08)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.15 (2022-08-01)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.14 (2022-07-11)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.13 (2022-07-05)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.12 (2022-06-29)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.11 (2022-06-16)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.10 (2022-06-07)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.9 (2022-05-26)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.8 (2022-05-25)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.7 (2022-05-17)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.6 (2022-05-16)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.5 (2022-05-09)

* **Bug Fix**: Fixes a bug in LoadDefaultConfig to correctly assign ConfigSources so all config resolvers have access to the config sources. This fixes the feature/ec2/imds client not having configuration applied via config.LoadOptions such as EC2IMDSClientEnableState. PR [#1682](https://github.com/aws/aws-sdk-go-v2/pull/1682)

# v1.15.4 (2022-04-25)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.3 (2022-03-30)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.2 (2022-03-24)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.1 (2022-03-23)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.0 (2022-03-08)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.14.0 (2022-02-24)

* **Feature**: Adds support for loading RetryMaxAttempts and RetryMod from the environment and shared configuration files. These parameters drive how the SDK's API client will initialize its default retryer, if custome retryer has not been specified. See [config](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config) module and [aws.Config](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws#Config) for more information about and how to use these new options.
* **Feature**: Adds support for the `ca_bundle` parameter in shared config and credentials files. The usage of the file is the same as environment variable, `AWS_CA_BUNDLE`, but sourced from shared config. Fixes [#1589](https://github.com/aws/aws-sdk-go-v2/issues/1589)
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.13.1 (2022-01-28)

* **Bug Fix**: Fixes LoadDefaultConfig handling of errors returned by passed in functional options. Previously errors returned from the LoadOptions passed into LoadDefaultConfig were incorrectly ignored. [#1562](https://github.com/aws/aws-sdk-go-v2/pull/1562). Thanks to [Pinglei Guo](https://github.com/pingleig) for submitting this PR.
* **Bug Fix**: Fixes the SDK's handling of `duration_sections` in the shared credentials file or specified in multiple shared config and shared credentials files under the same profile. [#1568](https://github.com/aws/aws-sdk-go-v2/pull/1568). Thanks to [Amir Szekely](https://github.com/kichik) for help reproduce this bug.
* **Bug Fix**: Updates `config` module to use os.UserHomeDir instead of hard coded environment variable for OS. [#1563](https://github.com/aws/aws-sdk-go-v2/pull/1563)
* **Dependency Update**: Updated to the latest SDK module versions

# v1.13.0 (2022-01-14)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.0 (2022-01-07)

* **Feature**: Add load option for CredentialCache. Adds a new member to the LoadOptions struct, CredentialsCacheOptions. This member allows specifying a function that will be used to configure the CredentialsCache. The CredentialsCacheOptions will only be used if the configuration loader will wrap the underlying credential provider in the CredentialsCache.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.1 (2021-12-21)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.0 (2021-12-02)

* **Feature**: Add support for specifying `EndpointResolverWithOptions` on `LoadOptions`, and associated `WithEndpointResolverWithOptions`.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.10.3 (2021-11-30)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.10.2 (2021-11-19)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.10.1 (2021-11-12)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.10.0 (2021-11-06)

* **Feature**: The SDK now supports configuration of FIPS and DualStack endpoints using environment variables, shared configuration, or programmatically.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.9.0 (2021-10-21)

* **Feature**: Updated  to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.8.3 (2021-10-11)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.8.2 (2021-09-17)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.8.1 (2021-09-10)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.8.0 (2021-09-02)

* **Feature**: Add support for S3 Multi-Region Access Point ARNs.

# v1.7.0 (2021-08-27)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.6.1 (2021-08-19)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.6.0 (2021-08-04)

* **Feature**: adds error handling for defered close calls
* **Dependency Update**: Updated `github.com/aws/smithy-go` to latest version.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.5.0 (2021-07-15)

* **Feature**: Support has been added for EC2 IPv6-enabled Instance Metadata Service Endpoints.
* **Dependency Update**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.4.1 (2021-07-01)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.4.0 (2021-06-25)

* **Feature**: Adds configuration setting for enabling endpoint discovery.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.3.0 (2021-05-20)

* **Feature**: SSO credentials can now be defined alongside other credential providers within the same configuration profile.
* **Bug Fix**: Profile names were incorrectly normalized to lower-case, which could result in unexpected profile configurations.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.2.0 (2021-05-14)

* **Feature**: Constant has been added to modules to enable runtime version inspection for reporting.
* **Dependency Update**: Updated to the latest SDK module versions

