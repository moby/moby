# runtime [![Build Status](https://github.com/go-openapi/runtime/actions/workflows/go-test.yml/badge.svg)](https://github.com/go-openapi/runtime/actions?query=workflow%3A"go+test") [![codecov](https://codecov.io/gh/go-openapi/runtime/branch/master/graph/badge.svg)](https://codecov.io/gh/go-openapi/runtime)

[![Slack Status](https://slackin.goswagger.io/badge.svg)](https://slackin.goswagger.io)
[![license](http://img.shields.io/badge/license-Apache%20v2-orange.svg)](https://raw.githubusercontent.com/go-openapi/runtime/master/LICENSE) 
[![Go Reference](https://pkg.go.dev/badge/github.com/go-openapi/runtime.svg)](https://pkg.go.dev/github.com/go-openapi/runtime)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-openapi/runtime)](https://goreportcard.com/report/github.com/go-openapi/runtime)

# go OpenAPI toolkit runtime

The runtime component for use in code generation or as untyped usage.

## Release notes

### v0.29.0

**New with this release**:

* upgraded to `go1.24` and modernized the code base accordingly
* updated all dependencies, and removed an noticable indirect dependency (e.g. `mailru/easyjson`)
* **breaking change** no longer imports `opentracing-go` (#365).
    * the `WithOpentracing()` method now returns an opentelemetry transport
    * for users who can't transition to opentelemetry, the previous behavior
      of `WithOpentracing` delivering an opentracing transport is provided by a separate
      module `github.com/go-openapi/runtime/client-middleware/opentracing`.
* removed direct dependency to `gopkg.in/yaml.v3`, in favor of `go.yaml.in/yaml/v3` (an indirect
  test dependency to the older package is still around)
* technically, the repo has evolved to a mono-repo, multiple modules structures (2 go modules
  published), with CI adapted accordingly

**What coming next?**

Moving forward, we want to :

* [ ] continue narrowing down the scope of dependencies:
  * yaml support in an independent module
  * introduce more up-to-date support for opentelemetry as a separate module that evolves
    independently from the main package (to avoid breaking changes, the existing API
    will remain maintained, but evolve at a slower pace than opentelemetry).
* [ ] fix a few known issues with some file upload requests (e.g. #286)

## Licensing

This library ships under the [SPDX-License-Identifier: Apache-2.0](./LICENSE).
