# ttrpc OpenTelemetry Instrumentation

This golang package implements OpenTelemetry instrumentation support for
ttrpc. It can be used to automatically generate OpenTelemetry trace spans
for RPC methods called on the ttrpc client side and served on the ttrpc
server side.

# Usage

Instrumentation is provided by two interceptors, one to enable instrumentation
for unary clients and another for enabling instrumentation for unary servers.
These interceptors can be passed as ttrpc.ClientOpts and ttrpc.ServerOpt to
ttrpc during client and server creation with code like this:

```golang

   import (
       "github.com/containerd/ttrpc"
       "github.com/containerd/otelttrpc"
   )

   // on the client side
   ...
   client := ttrpc.NewClient(
       conn,
       ttrpc.UnaryClientInterceptor(
           otelttrpc.UnaryClientInterceptor(),
       ),
   )

   // and on the server side
   ...
   server, err := ttrpc.NewServer(
       ttrpc.WithUnaryServerInterceptor(
           otelttrpc.UnaryServerInterceptor(),
       ),
   )
```

Once enabled, the interceptors generate trace Spans for all called and served
unary method calls. If the rest of the code is properly set up to collect and
export tracing data to opentelemetry, these spans should show up as part of
the collected traces.

For a more complete example see the [sample client](example/client/main.go)
and the [sample server](example/server/main.go) code.

# Limitations

Currently only unary client and unary server methods can be instrumented.
Support for streaming interfaces is yet to be implemented.

# Project details

otelttrpc is a containerd sub-project, licensed under the [Apache 2.0 license](./LICENSE).
As a containerd sub-project, you will find the:
 * [Project governance](https://github.com/containerd/project/blob/main/GOVERNANCE.md),
 * [Maintainers](https://github.com/containerd/project/blob/main/MAINTAINERS),
 * and [Contributing guidelines](https://github.com/containerd/project/blob/main/CONTRIBUTING.md)

information in our [`containerd/project`](https://github.com/containerd/project) repository.
