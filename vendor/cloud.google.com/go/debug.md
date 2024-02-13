# Debugging tips and tricks

While working with the Go Client libraries you may run into some situations
where you need a deeper level of understanding about what is going on in order
to solve your problem. Here are some tips and tricks that you can use in these
cases. *Note* that many of the tips in this document will have a performance
impact and are therefore not recommended for sustained production use. Use these
tips locally or in production for a *limited time* to help get a better
understanding of what is going on.

## HTTP based clients

All of our auto-generated clients have a constructor to create a client that
uses HTTP/JSON instead of gRPC. Additionally a couple of our hand-written
clients like Storage and Bigquery are also HTTP based. Here are some tips for
debugging these clients.

### Try setting Go's HTTP debug variable

Try setting the following environment variable for verbose Go HTTP logging:
GODEBUG=http2debug=1. To read more about this feature please see the godoc for
[net/http](https://pkg.go.dev/net/http).

*WARNING*: Enabling this debug variable will log headers and payloads which may
contain private information.

### Add in your own logging with an HTTP middleware

You may want to add in your own logging around HTTP requests. One way to do this
is to register a custom HTTP client with a logging transport built in. Here is
an example of how you would do this with the storage client.

*WARNING*: Adding this middleware will log headers and payloads which may
contain private information.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "net/http/httputil"

    "cloud.google.com/go/storage"
    "google.golang.org/api/iterator"
    "google.golang.org/api/option"
    htransport "google.golang.org/api/transport/http"
)

type loggingRoundTripper struct {
    rt http.RoundTripper
}

func (d loggingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
    // Will create a dump of the request and body.
    dump, err := httputil.DumpRequest(r, true)
    if err != nil {
        log.Println("error dumping request")
    }
    log.Printf("%s", dump)
    return d.rt.RoundTrip(r)
}

func main() {
    ctx := context.Background()

    // Create a transport with authentication built-in detected with
    // [ADC](https://google.aip.dev/auth/4110). Note you will have to pass any
    // required scoped for the client you are using.
    trans, err := htransport.NewTransport(ctx,
        http.DefaultTransport,
        option.WithScopes(storage.ScopeFullControl),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Embed customized transport into an HTTP client.
    hc := &http.Client{
        Transport: loggingRoundTripper{rt: trans},
    }

    // Supply custom HTTP client for use by the library.
    client, err := storage.NewClient(ctx, option.WithHTTPClient(hc))
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    // Use the client
}
```

## gRPC based clients

### Try setting grpc-go's debug variables

Try setting the following environment variables for grpc-go:
`GRPC_GO_LOG_VERBOSITY_LEVEL=99` `GRPC_GO_LOG_SEVERITY_LEVEL=info`. These are
good for diagnosing connection level failures. For more information please see
[grpc-go's debug documentation](https://pkg.go.dev/google.golang.org/grpc/examples/features/debugging#section-readme).

### Add in your own logging with a gRPC interceptors

You may want to add in your own logging around gRPC requests. One way to do this
is to register a custom interceptor that adds logging. Here is
an example of how you would do this with the secretmanager client. Note this
example registers a UnaryClientInterceptor but you may want/need to register
a StreamClientInterceptor instead-of/as-well depending on what kinds of
RPCs you are calling.

*WARNING*: Adding this interceptor will log metadata and payloads which may
contain private information.

```go
package main

import (
    "context"
    "log"

    secretmanager "cloud.google.com/go/secretmanager/apiv1"
    "google.golang.org/api/option"
    "google.golang.org/grpc"
    "google.golang.org/grpc/metadata"
    "google.golang.org/protobuf/encoding/protojson"
    "google.golang.org/protobuf/reflect/protoreflect"
)

func loggingUnaryInterceptor() grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        err := invoker(ctx, method, req, reply, cc, opts...)
        log.Printf("Invoked method: %v", method)
        md, ok := metadata.FromOutgoingContext(ctx)
        if ok {
            log.Println("Metadata:")
            for k, v := range md {
                log.Printf("Key: %v, Value: %v", k, v)
            }
        }
        reqb, merr := protojson.Marshal(req.(protoreflect.ProtoMessage))
        if merr == nil {
            log.Printf("Request: %s", reqb)
        }
        return err
    }
}

func main() {
    ctx := context.Background()
    // Supply custom gRPC interceptor for use by the client.
    client, err := secretmanager.NewClient(ctx,
        option.WithGRPCDialOption(grpc.WithUnaryInterceptor(loggingUnaryInterceptor())),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    // Use the client
}
```
