# Instrumentation

Code contained in this directory contains instrumentation for 3rd-party Go packages and some packages from the standard library.

## Instrumentation Packages

The following instrumentation packages are provided for popular Go packages and use-cases.

| Instrumentation Package | Metrics | Traces |
| :---------------------: | :-----: | :----: |
| [github.com/astaxie/beego](./github.com/astaxie/beego/otelbeego) | ✓ | ✓ |
| [github.com/bradfitz/gomemcache](./github.com/bradfitz/gomemcache/memcache/otelmemcache) |  | ✓ |
| [github.com/emicklei/go-restful](./github.com/emicklei/go-restful/otelrestful) |  | ✓ |
| [github.com/gin-gonic/gin](./github.com/gin-gonic/gin/otelgin) |  | ✓ |
| [github.com/go-kit/kit](./github.com/go-kit/kit/otelkit) |  | ✓ |
| [github.com/gocql/gocql](./github.com/gocql/gocql/otelgocql) | ✓ | ✓ |
| [github.com/gorilla/mux](./github.com/gorilla/mux/otelmux) |  | ✓ |
| [github.com/labstack/echo](./github.com/labstack/echo/otelecho) |  | ✓ |
| [github.com/Shopify/sarama](./github.com/Shopify/sarama/otelsarama) |  | ✓ |
| [go.mongodb.org/mongo-driver](./go.mongodb.org/mongo-driver/mongo/otelmongo) |  | ✓ |
| [google.golang.org/grpc](./google.golang.org/grpc/otelgrpc) |  | ✓ |
| [gopkg.in/macaron.v1](./gopkg.in/macaron.v1/otelmacaron) |  | ✓ |
| [host](./host) | ✓ |  |
| [net/http](./net/http/otelhttp) | ✓ | ✓ |
| [net/http/httptrace](./net/http/httptrace/otelhttptrace) |  | ✓ |
| [runtime](./runtime) | ✓ |  |


Additionally, these are the known instrumentation packages that exist outside of this repository for popular Go packages.

| Package Name | Documentation | Notes |
| :----------: | :-----------: | :---: |
| [`github.com/go-redis/redis/v8/redisext`](https://github.com/go-redis/redis/blob/v8.0.0-beta.5/redisext/otel.go) | [Go Docs](https://pkg.go.dev/github.com/go-redis/redis/v8@v8.0.0-beta.5.0.20200614113957-5b4d00c217b0/redisext?tab=doc) | Trace only; add the hook found [here](https://github.com/go-redis/redis/blob/v8.0.0-beta.5/redisext/otel.go) to your go-redis client. |

## Organization

In order to ensure the maintainability and discoverability of instrumentation packages, the following guidelines MUST be followed.

### Packaging

All instrumentation packages SHOULD be of the form:

```
go.opentelemetry.io/contrib/instrumentation/{IMPORT_PATH}/otel{PACKAGE_NAME}
```

Where the [`{IMPORT_PATH}`](https://golang.org/ref/spec#ImportPath) and [`{PACKAGE_NAME}`](https://golang.org/ref/spec#PackageName) are the standard Go identifiers for the package being instrumented.

For example:

- `go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux`
- `go.opentelemetry.io/contrib/instrumentation/gopkg.in/macaron.v1/otelmacaron`
- `go.opentelemetry.io/contrib/instrumentation/database/sql/otelsql`

Exceptions to this rule do exist.
For example, the [runtime](./runtime) instrumentation does not instrument a Go package and does not fit this structure.

### Contents

All instrumentation packages MUST adhere to [the projects' contributing guidelines](../CONTRIBUTING.md).
Additionally the following guidelines for package composition need to be followed.

- All instrumentation packages MUST be a Go package.
   Therefore, an appropriately configured `go.mod` and `go.sum` need to exist for each package.
- To help understand the instrumentation a Go package documentation SHOULD be included.
   This documentation SHOULD be in a dedicated `doc.go` file if the package is more than one file.
   It SHOULD contain useful information like what the purpose of the instrumentation is, how to use it, and any compatibility restrictions that might exist. 
- Examples of how to actually use the instrumentation SHOULD be included.
- All instrumentation packages MUST provide an option to accept a `TracerProvider` if it uses a Tracer, a `MeterProvider` if it uses a Meter, and `Propagators` if it handles any context propagation.
  Also, packages MUST use the default `TracerProvider`, `MeterProvider`, and `Propagators` supplied by the `global` package if no optional one is provided.
- All instrumentation packages MUST NOT provide an option to accept a `Tracer` or `Meter`.
- All instrumentation packages MUST create any used `Tracer` or `Meter` with a name matching the instrumentation package name.
- All instrumentation packages MUST create any used `Tracer` or `Meter` with a semantic version corresponding to the the version of this repository.
