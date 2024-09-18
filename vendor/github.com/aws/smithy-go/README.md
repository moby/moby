## Smithy Go

[![Go Build Status](https://github.com/aws/smithy-go/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/aws/smithy-go/actions/workflows/go.yml)[![Codegen Build Status](https://github.com/aws/smithy-go/actions/workflows/codegen.yml/badge.svg?branch=main)](https://github.com/aws/smithy-go/actions/workflows/codegen.yml)

[Smithy](https://smithy.io/) code generators for Go.

**WARNING: All interfaces are subject to change.**

## Can I use this?

In order to generate a usable smithy client you must provide a [protocol definition](https://github.com/aws/smithy-go/blob/main/codegen/smithy-go-codegen/src/main/java/software/amazon/smithy/go/codegen/integration/ProtocolGenerator.java),
such as [AWS restJson1](https://smithy.io/2.0/aws/protocols/aws-restjson1-protocol.html),
in order to generate transport mechanisms and serialization/deserialization
code ("serde") accordingly.

The code generator does not currently support any protocols out of the box,
therefore the useability of this project on its own is currently limited.
Support for all [AWS protocols](https://smithy.io/2.0/aws/protocols/index.html)
exists in [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2). We are
tracking the movement of those out of the SDK into smithy-go in
[#458](https://github.com/aws/smithy-go/issues/458), but there's currently no
timeline for doing so.

## License

This project is licensed under the Apache-2.0 License.

