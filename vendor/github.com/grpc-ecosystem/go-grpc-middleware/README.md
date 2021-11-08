# Go gRPC Middleware

[![Travis Build](https://travis-ci.org/grpc-ecosystem/go-grpc-middleware.svg?branch=master)](https://travis-ci.org/grpc-ecosystem/go-grpc-middleware)
[![Go Report Card](https://goreportcard.com/badge/github.com/grpc-ecosystem/go-grpc-middleware)](https://goreportcard.com/report/github.com/grpc-ecosystem/go-grpc-middleware)
[![GoDoc](http://img.shields.io/badge/GoDoc-Reference-blue.svg)](https://godoc.org/github.com/grpc-ecosystem/go-grpc-middleware)
[![SourceGraph](https://sourcegraph.com/github.com/grpc-ecosystem/go-grpc-middleware/-/badge.svg)](https://sourcegraph.com/github.com/grpc-ecosystem/go-grpc-middleware/?badge)
[![codecov](https://codecov.io/gh/grpc-ecosystem/go-grpc-middleware/branch/master/graph/badge.svg)](https://codecov.io/gh/grpc-ecosystem/go-grpc-middleware)
[![Apache 2.0 License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![quality: production](https://img.shields.io/badge/quality-production-orange.svg)](#status)
[![Slack](https://img.shields.io/badge/slack-%23grpc--middleware-brightgreen)](https://slack.com/share/IRUQCFC23/9Tm7hxRFVKKNoajQfMOcUiIk/enQtODc4ODI4NTIyMDcxLWM5NDA0ZTE4Njg5YjRjYWZkMTI5MzQwNDY3YzBjMzE1YzdjOGM5ZjI1NDNiM2JmNzI2YjM5ODE5OTRiNTEyOWE)

[gRPC Go](https://github.com/grpc/grpc-go) Middleware: interceptors, helpers, utilities.

## Middleware

[gRPC Go](https://github.com/grpc/grpc-go) recently acquired support for
Interceptors, i.e. [middleware](https://medium.com/@matryer/writing-middleware-in-golang-and-how-go-makes-it-so-much-fun-4375c1246e81#.gv7tdlghs) 
that is executed either on the gRPC Server before the request is passed onto the user's application logic, or on the gRPC client around the user call. It is a perfect way to implement
common patterns: auth, logging, message, validation, retries or monitoring.

These are generic building blocks that make it easy to build multiple microservices easily.
The purpose of this repository is to act as a go-to point for such reusable functionality. It contains
some of them itself, but also will link to useful external repos.

`grpc_middleware` itself provides support for chaining interceptors, here's an example:

```go
import "github.com/grpc-ecosystem/go-grpc-middleware"

myServer := grpc.NewServer(
    grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
        grpc_recovery.StreamServerInterceptor(),
        grpc_ctxtags.StreamServerInterceptor(),
        grpc_opentracing.StreamServerInterceptor(),
        grpc_prometheus.StreamServerInterceptor,
        grpc_zap.StreamServerInterceptor(zapLogger),
        grpc_auth.StreamServerInterceptor(myAuthFunction),
    )),
    grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
        grpc_recovery.UnaryServerInterceptor(),
        grpc_ctxtags.UnaryServerInterceptor(),
        grpc_opentracing.UnaryServerInterceptor(),
        grpc_prometheus.UnaryServerInterceptor,
        grpc_zap.UnaryServerInterceptor(zapLogger),
        grpc_auth.UnaryServerInterceptor(myAuthFunction),
    )),
)
```

## Interceptors

*Please send a PR to add new interceptors or middleware to this list*

#### Auth
   * [`grpc_auth`](auth) - a customizable (via `AuthFunc`) piece of auth middleware 

#### Logging
   * [`grpc_ctxtags`](tags/) - a library that adds a `Tag` map to context, with data populated from request body
   * [`grpc_zap`](logging/zap/) - integration of [zap](https://github.com/uber-go/zap) logging library into gRPC handlers.
   * [`grpc_logrus`](logging/logrus/) - integration of [logrus](https://github.com/sirupsen/logrus) logging library into gRPC handlers.
   * [`grpc_kit`](logging/kit/) - integration of [go-kit](https://github.com/go-kit/kit/tree/master/log) logging library into gRPC handlers.
   * [`grpc_grpc_logsettable`](logging/settable/) - a wrapper around `grpclog.LoggerV2` that allows to replace loggers in runtime (thread-safe).

#### Monitoring
   * [`grpc_prometheus`⚡](https://github.com/grpc-ecosystem/go-grpc-prometheus) - Prometheus client-side and server-side monitoring middleware
   * [`otgrpc`⚡](https://github.com/grpc-ecosystem/grpc-opentracing/tree/master/go/otgrpc) - [OpenTracing](http://opentracing.io/) client-side and server-side interceptors
   * [`grpc_opentracing`](tracing/opentracing) - [OpenTracing](http://opentracing.io/) client-side and server-side interceptors with support for streaming and handler-returned tags

#### Client
   * [`grpc_retry`](retry/) - a generic gRPC response code retry mechanism, client-side middleware

#### Server
   * [`grpc_validator`](validator/) - codegen inbound message validation from `.proto` options
   * [`grpc_recovery`](recovery/) - turn panics into gRPC errors
   * [`ratelimit`](ratelimit/) - grpc rate limiting by your own limiter


## Status

This code has been running in *production* since May 2016 as the basis of the gRPC micro services stack at [Improbable](https://improbable.io).

Additional tooling will be added, and contributions are welcome.

## License

`go-grpc-middleware` is released under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
