# OpenTelemetry-Go Contrib

[![build_and_test](https://github.com/open-telemetry/opentelemetry-go-contrib/workflows/build_and_test/badge.svg)](https://github.com/open-telemetry/opentelemetry-go-contrib/actions?query=workflow%3Abuild_and_test+branch%3Amain)
[![codecov.io](https://codecov.io/gh/open-telemetry/opentelemetry-go-contrib/coverage.svg?branch=main)](https://app.codecov.io/gh/open-telemetry/opentelemetry-go-contrib?branch=main)
[![Docs](https://godoc.org/go.opentelemetry.io/contrib?status.svg)](https://pkg.go.dev/go.opentelemetry.io/contrib)
[![Go Report Card](https://goreportcard.com/badge/go.opentelemetry.io/contrib)](https://goreportcard.com/report/go.opentelemetry.io/contrib)
[![Slack](https://img.shields.io/badge/slack-@cncf/otel--go-brightgreen.svg?logo=slack)](https://cloud-native.slack.com/archives/C01NPAXACKT)

Collection of 3rd-party instrumentation and exporters for [OpenTelemetry-Go](https://github.com/open-telemetry/opentelemetry-go).

## Contents

- [Instrumentation](./instrumentation/): Packages providing OpenTelemetry instrumentation for 3rd-party libraries.
- [Exporters](./exporters/): Packages providing OpenTelemetry exporters for 3rd-party telemetry systems.
- [Propagators](./propagators/): Packages providing OpenTelemetry context propagators for 3rd-party propagation formats.
- [Detectors](./detectors/): Packages providing OpenTelemetry resource detectors for 3rd-party cloud computing environments.

## Project Status

This project is currently in a pre-GA phase. Our progress towards a GA release
candidate is tracked in [this project
board](https://github.com/orgs/open-telemetry/projects/5).

### Compatibility

This project is tested on the following systems.

| OS      | Go Version | Architecture |
| ------- | ---------- | ------------ |
| Ubuntu  | 1.16       | amd64        |
| Ubuntu  | 1.15       | amd64        |
| Ubuntu  | 1.16       | 386          |
| Ubuntu  | 1.15       | 386          |

While this project should work for other systems, no compatibility guarantees
are made for those systems currently.

## Contributing

For information on how to contribute, consult [the contributing guidelines](./CONTRIBUTING.md)
