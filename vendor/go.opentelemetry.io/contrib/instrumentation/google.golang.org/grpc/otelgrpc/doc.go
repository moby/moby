// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package otelgrpc is the instrumentation library for [google.golang.org/grpc]

For now you can instrument your program which use [google.golang.org/grpc] in two ways:

  - by [grpc.UnaryClientInterceptor], [grpc.UnaryServerInterceptor], [grpc.StreamClientInterceptor], [grpc.StreamServerInterceptor]
  - by [stats.Handler]

Notice: Do not use both interceptors and [stats.Handler] at the same time! If so, you will get duplicated spans and the parent/child relationships between spans will also be broken.

We strongly still recommand you to use [stats.Handler], mainly for two reasons:

Functional advantages: [stats.Handler] has more information for user to build more flexible and granular metric, for example

  - multiple different types of represent "data length": In [stats.InPayload], there exists "Length", "CompressedLength", "WireLength" to denote the size of uncompressed, compressed payload data, with or without framing data. But in interceptors, we can only got uncompressed data, and this feature is also removed due to performance problem.

  - more accurate timestamp: [stats.InPayload]'s "RecvTime" and [stats.OutPayload]'s "SentTime" records more accurate timestamp that server got and sent the message, the timestamp recorded by interceptors depends on the location of this interceptors in the total interceptor chain.

  - some other use cases: for example, catch failure of decoding message.

Performance advantages: If too many interceptors are registered in a service, the interceptor chain can become too long, which increases the latency and processing time of the entire RPC call.

[stats.Handler]: https://pkg.go.dev/google.golang.org/grpc/stats#Handler
[grpc.UnaryClientInterceptor]: https://pkg.go.dev/google.golang.org/grpc#UnaryClientInterceptor
[grpc.UnaryServerInterceptor]: https://pkg.go.dev/google.golang.org/grpc#UnaryServerInterceptor
[grpc.StreamClientInterceptor]: https://pkg.go.dev/google.golang.org/grpc#StreamClientInterceptor
[grpc.StreamServerInterceptor]: https://pkg.go.dev/google.golang.org/grpc#StreamServerInterceptor
[stats.OutPayload]: https://pkg.go.dev/google.golang.org/grpc/stats#OutPayload
[stats.InPayload]: https://pkg.go.dev/google.golang.org/grpc/stats#InPayload
*/
package otelgrpc // import "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
