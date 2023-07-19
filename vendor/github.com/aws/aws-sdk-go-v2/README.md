# AWS SDK for Go v2

[![Go Build status](https://github.com/aws/aws-sdk-go-v2/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/aws/aws-sdk-go-v2/actions/workflows/go.yml)[![Codegen Build status](https://github.com/aws/aws-sdk-go-v2/actions/workflows/codegen.yml/badge.svg?branch=main)](https://github.com/aws/aws-sdk-go-v2/actions/workflows/codegen.yml) [![SDK Documentation](https://img.shields.io/badge/SDK-Documentation-blue)](https://aws.github.io/aws-sdk-go-v2/docs/) [![Migration Guide](https://img.shields.io/badge/Migration-Guide-blue)](https://aws.github.io/aws-sdk-go-v2/docs/migrating/) [![API Reference](https://img.shields.io/badge/api-reference-blue.svg)](https://pkg.go.dev/mod/github.com/aws/aws-sdk-go-v2) [![Apache V2 License](https://img.shields.io/badge/license-Apache%20V2-blue.svg)](https://github.com/aws/aws-sdk-go-v2/blob/main/LICENSE.txt)

`aws-sdk-go-v2` is the v2 AWS SDK for the Go programming language.

The v2 SDK requires a minimum version of `Go 1.15`.

Check out the [release notes](https://github.com/aws/aws-sdk-go-v2/blob/main/CHANGELOG.md) for information about the latest bug
fixes, updates, and features added to the SDK.

Jump To:
* [Getting Started](#getting-started)
* [Getting Help](#getting-help)
* [Contributing](#feedback-and-contributing)
* [More Resources](#resources)

## Maintenance and support for SDK major versions

For information about maintenance and support for SDK major versions and their underlying dependencies, see the
following in the AWS SDKs and Tools Shared Configuration and Credentials Reference Guide:

* [AWS SDKs and Tools Maintenance Policy](https://docs.aws.amazon.com/credref/latest/refdocs/maint-policy.html)
* [AWS SDKs and Tools Version Support Matrix](https://docs.aws.amazon.com/credref/latest/refdocs/version-support-matrix.html)

## Getting started
To get started working with the SDK setup your project for Go modules, and retrieve the SDK dependencies with `go get`.
This example shows how you can use the v2 SDK to make an API request using the SDK's [Amazon DynamoDB] client.

###### Initialize Project
```sh
$ mkdir ~/helloaws
$ cd ~/helloaws
$ go mod init helloaws
```
###### Add SDK Dependencies
```sh
$ go get github.com/aws/aws-sdk-go-v2/aws
$ go get github.com/aws/aws-sdk-go-v2/config
$ go get github.com/aws/aws-sdk-go-v2/service/dynamodb
```

###### Write Code
In your preferred editor add the following content to `main.go`

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func main() {
    // Using the SDK's default configuration, loading additional config
    // and credentials values from the environment variables, shared
    // credentials, and shared configuration files
    cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-west-2"))
    if err != nil {
        log.Fatalf("unable to load SDK config, %v", err)
    }

    // Using the Config value, create the DynamoDB client
    svc := dynamodb.NewFromConfig(cfg)

    // Build the request with its input parameters
    resp, err := svc.ListTables(context.TODO(), &dynamodb.ListTablesInput{
        Limit: aws.Int32(5),
    })
    if err != nil {
        log.Fatalf("failed to list tables, %v", err)
    }

    fmt.Println("Tables:")
    for _, tableName := range resp.TableNames {
        fmt.Println(tableName)
    }
}
```

###### Compile and Execute
```sh
$ go run .
Tables:
tableOne
tableTwo
```

## Getting Help

Please use these community resources for getting help. We use the GitHub issues
for tracking bugs and feature requests.

* Ask us a [question](https://github.com/aws/aws-sdk-go-v2/discussions/new?category=q-a) or open a [discussion](https://github.com/aws/aws-sdk-go-v2/discussions/new?category=general).
* If you think you may have found a bug, please open an [issue](https://github.com/aws/aws-sdk-go-v2/issues/new/choose).
* Open a support ticket with [AWS Support](http://docs.aws.amazon.com/awssupport/latest/user/getting-started.html).

This SDK implements AWS service APIs. For general issues regarding the AWS services and their limitations, you may also take a look at the [Amazon Web Services Discussion Forums](https://forums.aws.amazon.com/).

### Opening Issues

If you encounter a bug with the AWS SDK for Go we would like to hear about it.
Search the [existing issues][Issues] and see
if others are also experiencing the same issue before opening a new issue. Please
include the version of AWS SDK for Go, Go language, and OS you’re using. Please
also include reproduction case when appropriate.

The GitHub issues are intended for bug reports and feature requests. For help
and questions with using AWS SDK for Go please make use of the resources listed
in the [Getting Help](#getting-help) section.
Keeping the list of open issues lean will help us respond in a timely manner.

## Feedback and contributing

The v2 SDK will use GitHub [Issues] to track feature requests and issues with the SDK. In addition, we'll use GitHub [Projects] to track large tasks spanning multiple pull requests, such as refactoring the SDK's internal request lifecycle. You can provide feedback to us in several ways.

**GitHub issues**. To provide feedback or report bugs, file GitHub [Issues] on the SDK. This is the preferred mechanism to give feedback so that other users can engage in the conversation, +1 issues, etc. Issues you open will be evaluated, and included in our roadmap for the GA launch.

**Contributing**. You can open pull requests for fixes or additions to the AWS SDK for Go 2.0. All pull requests must be submitted under the Apache 2.0 license and will be reviewed by an SDK team member before being merged in. Accompanying unit tests, where possible, are appreciated.

## Resources

[SDK Developer Guide](https://aws.github.io/aws-sdk-go-v2/docs/) - Use this document to learn how to get started and
use the AWS SDK for Go V2.

[SDK Migration Guide](https://aws.github.io/aws-sdk-go-v2/docs/migrating/) - Use this document to learn how to migrate to V2 from the AWS SDK for Go.

[SDK API Reference Documentation](https://pkg.go.dev/mod/github.com/aws/aws-sdk-go-v2) - Use this
document to look up all API operation input and output parameters for AWS
services supported by the SDK. The API reference also includes documentation of
the SDK, and examples how to using the SDK, service client API operations, and
API operation require parameters.

[Service Documentation](https://aws.amazon.com/documentation/) - Use this
documentation to learn how to interface with AWS services. These guides are
great for getting started with a service, or when looking for more
information about a service. While this document is not required for coding,
services may supply helpful samples to look out for.

[Forum](https://forums.aws.amazon.com/forum.jspa?forumID=293) - Ask questions, get help, and give feedback

[Issues] - Report issues, submit pull requests, and get involved
  (see [Apache 2.0 License][license])

[Dep]: https://github.com/golang/dep
[Issues]: https://github.com/aws/aws-sdk-go-v2/issues
[Projects]: https://github.com/aws/aws-sdk-go-v2/projects
[CHANGELOG]: https://github.com/aws/aws-sdk-go-v2/blob/main/CHANGELOG.md
[Amazon DynamoDB]: https://aws.amazon.com/dynamodb/
[design]: https://github.com/aws/aws-sdk-go-v2/blob/main/DESIGN.md
[license]: http://aws.amazon.com/apache2.0/
