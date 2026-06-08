# Smithy Go

[![Go Build Status](https://github.com/aws/smithy-go/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/aws/smithy-go/actions/workflows/go.yml)[![Codegen Build Status](https://github.com/aws/smithy-go/actions/workflows/codegen.yml/badge.svg?branch=main)](https://github.com/aws/smithy-go/actions/workflows/codegen.yml)

[Smithy](https://smithy.io/) code generators for Go and the accompanying smithy-go runtime.

The smithy-go runtime requires a minimum version of Go 1.24.

**WARNING: All interfaces are subject to change.**

## :warning: Client codegen is unstable

The client code generator in this repository powers the aws-sdk-go-v2.
Arbitrary client generation, while technically possible, is in an early stage
of development:

* Generated clients are missing certain features that were originally
  implemented SDK-side (e.g. retries)
* There may be bugs
* The public APIs of generated clients may be unstable

If you are interested in using the client code generators, we encourage you to
experiment and share any feedback with us in an issue.

## Plugins

This repository implements the following Smithy build plugins:

| ID | GAV prefix | Description |
|----|------------|-------------|
| `go-codegen`        | `software.amazon.smithy.go:smithy-go-codegen` | Implements Go client code generation for Smithy models. |
| `go-server-codegen` | `software.amazon.smithy.go:smithy-go-codegen` | Implements Go server code generation for Smithy models. |
| `go-shape-codegen` | `software.amazon.smithy.go:smithy-go-codegen` | Implements Go shape code generation (types only) for Smithy models. |

**NOTE: Build plugins are not currently published to mavenCentral. You must publish to mavenLocal to make the build plugins visible to the Smithy CLI. The artifact version is currently fixed at 0.1.0.**

## `go-codegen`

### Configuration

[`GoSettings`](codegen/smithy-go-codegen/src/main/java/software/amazon/smithy/go/codegen/GoSettings.java)
contains all of the settings enabled from `smithy-build.json` and helper
methods and types. The up-to-date list of top-level properties enabled for
`go-client-codegen` can be found in `GoSettings::from()`.

| Setting         | Type    | Required | Description                                                                                                                 |
|-----------------|---------|----------|-----------------------------------------------------------------------------------------------------------------------------|
| `service`       | string  | yes      | The Shape ID of the service for which to generate the client.                                                               |
| `module`        | string  | yes      | Name of the module in `generated.json` (and `go.mod` if `generateGoMod` is enabled) and `doc.go`.                           |
| `generateGoMod` | boolean |          | Whether to generate a default `go.mod` file. The default value is `false`.                                                  |
| `goDirective`   | string  |          | [Go directive](https://go.dev/ref/mod#go-mod-file-go) of the module. The default value is the minimum supported Go version. |

### Supported protocols

The protocol a client uses is configured by the `Protocol` field on a client's
`Options`. The SDK will configure a default based on the protocol traits
applied to the modeled service.

| Protocol | Notes |
|----------|-------|
| [`smithy.protocols#rpcv2Cbor`](https://smithy.io/2.0/additional-specs/protocols/smithy-rpc-v2.html) | |
| [`aws.protocols#restJson1`](https://smithy.io/2.0/aws/protocols/aws-restjson1-protocol.html) | |
| [`aws.protocols#restXml`](https://smithy.io/2.0/aws/protocols/aws-restxml-protocol.html) | |
| [`aws.protocols#awsJson1_0`](https://smithy.io/2.0/aws/protocols/aws-json-1_0-protocol.html) | |
| [`aws.protocols#awsJson1_1`](https://smithy.io/2.0/aws/protocols/aws-json-1_1-protocol.html) | |
| [`aws.protocols#awsQuery`](https://smithy.io/2.0/aws/protocols/aws-query-protocol.html) | |
| [`aws.protocols#ec2Query`](https://smithy.io/2.0/aws/protocols/aws-ec2-query-protocol.html) | |

### Example

This example applies the `go-codegen` build plugin to the Smithy quickstart
example created from `smithy init`:

```json
{
  "version": "1.0",
  "sources": [
    "models"
  ],
  "maven": {
    "dependencies": [
      "software.amazon.smithy.go:smithy-go-codegen:0.1.0"
    ]
  },
  "plugins": {
    "go-codegen": {
      "service": "example.weather#Weather",
      "module": "github.com/example/weather",
      "generateGoMod": true,
      "goDirective": "1.24"
    }
  }
}
```

## `go-server-codegen`

This plugin is a work-in-progress and is currently undocumented.

## `go-shape-codegen`

This plugin is a work-in-progress and is currently undocumented.

## License

This project is licensed under the Apache-2.0 License.

