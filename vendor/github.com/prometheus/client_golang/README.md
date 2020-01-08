# Prometheus Go client library

[![Build Status](https://travis-ci.org/prometheus/client_golang.svg?branch=master)](https://travis-ci.org/prometheus/client_golang)
[![Go Report Card](https://goreportcard.com/badge/github.com/prometheus/client_golang)](https://goreportcard.com/report/github.com/prometheus/client_golang)
[![go-doc](https://godoc.org/github.com/prometheus/client_golang?status.svg)](https://godoc.org/github.com/prometheus/client_golang)

This is the [Go](http://golang.org) client library for
[Prometheus](http://prometheus.io). It has two separate parts, one for
instrumenting application code, and one for creating clients that talk to the
Prometheus HTTP API.

__This library requires Go1.9 or later.__

## Important note about releases, versioning, tagging, and stability

In this repository, we used to mostly ignore the many coming and going
dependency management tools for Go and instead wait for a tool that most of the
community would converge on. Our bet is that this tool has arrived now in the
form of [Go
Modules](https://github.com/golang/go/wiki/Modules#how-to-upgrade-and-downgrade-dependencies).

To make full use of what Go Modules are offering, the previous versioning
roadmap for this repository had to be changed. In particular, Go Modules
finally provide a way for incompatible versions of the same package to coexist
in the same binary. For that, however, the versions must be tagged with
different major versions of 1 or greater (following [Semantic
Versioning](https://semver.org/)). Thus, we decided to abandon the original
plan of introducing a lot of breaking changes _before_ releasing v1 of this
repository, mostly driven by the widespread use this repository already has and
the relatively stable state it is in.

To leverage the mechanism Go Modules offers for a transition between major
version, the current plan is the following:

- The v0.9.x series of releases will see a small number of bugfix releases to
  deal with a few remaining minor issues (#543, #542, #539).
- After that, all features currently marked as _deprecated_ will be removed,
  and the result will be released as v1.0.0.
- The planned breaking changes previously gathered as part of the v0.10
  milestone will now go into the v2 milestone. The v2 development happens in a
  [separate branch](https://github.com/prometheus/client_golang/tree/dev-v2)
  for the time being. v2 releases off that branch will happen once sufficient
  stability is reached. v1 and v2 will coexist for a while to enable a
  convenient transition.
- The API client in prometheus/client_golang/api/… is still considered
  experimental. While it will be tagged alongside the rest of the code
  according to the plan above, we cannot strictly guarantee semver semantics
  for it.

## Instrumenting applications

[![code-coverage](http://gocover.io/_badge/github.com/prometheus/client_golang/prometheus)](http://gocover.io/github.com/prometheus/client_golang/prometheus) [![go-doc](https://godoc.org/github.com/prometheus/client_golang/prometheus?status.svg)](https://godoc.org/github.com/prometheus/client_golang/prometheus)

The
[`prometheus` directory](https://github.com/prometheus/client_golang/tree/master/prometheus)
contains the instrumentation library. See the
[guide](https://prometheus.io/docs/guides/go-application/) on the Prometheus
website to learn more about instrumenting applications.

The
[`examples` directory](https://github.com/prometheus/client_golang/tree/master/examples)
contains simple examples of instrumented code.

## Client for the Prometheus HTTP API

[![code-coverage](http://gocover.io/_badge/github.com/prometheus/client_golang/api/prometheus/v1)](http://gocover.io/github.com/prometheus/client_golang/api/prometheus/v1) [![go-doc](https://godoc.org/github.com/prometheus/client_golang/api/prometheus?status.svg)](https://godoc.org/github.com/prometheus/client_golang/api)

The
[`api/prometheus` directory](https://github.com/prometheus/client_golang/tree/master/api/prometheus)
contains the client for the
[Prometheus HTTP API](http://prometheus.io/docs/querying/api/). It allows you
to write Go applications that query time series data from a Prometheus
server. It is still in alpha stage.

## Where is `model`, `extraction`, and `text`?

The `model` packages has been moved to
[`prometheus/common/model`](https://github.com/prometheus/common/tree/master/model).

The `extraction` and `text` packages are now contained in
[`prometheus/common/expfmt`](https://github.com/prometheus/common/tree/master/expfmt).

## Contributing and community

See the [contributing guidelines](CONTRIBUTING.md) and the
[Community section](http://prometheus.io/community/) of the homepage.
