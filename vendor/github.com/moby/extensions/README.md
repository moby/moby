# Extensions

> [!WARNING]
> ⚠️ This project is a work in progress.

Moby extensions are one model for adding engine behavior.
They replace the older per-feature plugin systems for network, volume, and log drivers.

## Mental model

- A **point** is a versioned, namespaced Go interface and its message types.
- An **extension** implements one or more points.
  It can be compiled into the daemon or run as a separate binary; the point and its callers do not change.
- The host registers extensions, resolves typed dependencies, initializes them in dependency order, and routes calls through points.
- Wire code is generated from the Go contract.
  The generated proto and adapters let a separate-binary provider be written in another language.
- An ordinary point can also publish its generated gRPC service on `docker.sock`.
  Publication is opt-in and uses the same point contract; it does not create a second source contract.

## Principles

- **Everything is a point.**
  Hooks are points called during engine flows, and extension-to-extension calls use the same provider and routing model.
- **Placement is transparent.**
  The same point contract works in-process or in a separate process; the runtime chooses direct Go or gRPC transport.
- **Dependencies are typed and explicit.**
  The broker resolves declared point or extension dependencies before initialization; importing a package does not activate it.
- **Fan-out order is not a contract.**
  A point that needs ordering defines its own phases or ordering rules.
- **Socket publication is opt-in.**
  An extension publishes only the ordinary points it explicitly exposes, and those services must enforce their own access control.
- **Legacy plugins are replaced.**
  Network, volume, and log drivers become extension points rather than separate plugin systems.

## Ordinary Point publication

The common contract is an ordinary `extensions.DefinePoint` or
`extensions.DefineSinglePoint`. The interface name determines the gRPC service
name:

```go
package greeterv0

import (
	"context"

	"github.com/moby/extensions"
)

type Greeter interface {
	Greet(context.Context, *HelloRequest) (*HelloReply, error)
}

type HelloRequest struct {
	Name string `pb:"1"`
}

type HelloReply struct {
	Message string `pb:"1"`
}

var Point = extensions.DefinePoint[Greeter](
	"org.mobyproject.extension.example.greeter.v0",
)
```

`mobyextgen` infers `<PointID>.<InterfaceName>`, here
`org.mobyproject.extension.example.greeter.v0.Greeter`. The interface name is
part of the wire identity, so keep it stable within a Point version.

Generated Point packages provide `ServerPoint`, `ClientPoint`, and handwritten
`NewClient(conn)`. An extension implements an ordinary Point and separately
offers it for Host-controlled publication:

```go
var Extension = extensions.New(extensions.Declaration{
	ID: "org.example.greeter.v1",
	Providers: []extensions.Provider{
		greeterv0.Point.Provide(greeter{}),
		servicev0.Offer(greeterv0.Point),
	},
})
```

One offer may name a subset of several implemented Points:

```go
Providers: []extensions.Provider{
	foov1.Point.Provide(fooImpl{}),
	barv1.Point.Provide(barImpl{}),
	servicev0.Offer(foov1.Point),
}
```

An offer is not authorization. The Host defaults to deny and applies policy per
extension and Point:

```go
AllowPublication: host.PublicationPolicyFunc(func(extension extensions.ExtensionID, point extensions.PointID) bool {
	return extension == "org.example.greeter.v1" && point == greeterv0.Point.ID()
}),
```

For a separate binary, pass each implemented Point's generated adapter at the
process composition boundary:

```go
sdk.Main(extension, greeterpb.ServerPoint)
```

For an in-process extension, supply the adapter through `host.Options.PointServers`.
An in-process provider has no private SDK gRPC server, so the Host needs this
generated registration logic to translate protobuf requests into calls on the
Go implementation.
Offered process Points need `ClientPoint` wiring only when the daemon also calls
them internally.
External callers use the generated handwritten client:

```go
client := greeterpb.NewClient(hostConn)
reply, err := client.Greet(ctx, &greeterv0.HelloRequest{Name: "world"})
```

The current backend is gRPC. For a launched extension, the SDK registers the
generated service, the daemon records its fully qualified name, and the proxy
forwards the raw gRPC stream, including metadata and status. Arbitrary raw gRPC
publication is not part of the extension API.

## Glossary

| Term | Definition |
|---|---|
| **Point** | A versioned, namespaced Go interface and message types implemented by extensions. A breaking change gets a new point version; `.v0` is experimental. |
| **Extension** | A deployable unit, built into the daemon or run as a separate binary, that declares providers, dependencies, and lifecycle. |
| **Provider** | An extension's implementation of one point. It has no separate id and is identified by its extension id; an extension implements a point at most once. |
| **ClientPoint** | Generated host wiring that turns an extension connection into a provider of an ordinary point. |
| **ServerPoint** | Generated SDK or callback wiring that serves an ordinary point's gRPC service. |
| **Publication** | A Host policy allowing an extension-offered ordinary Point to become externally reachable. |
| **Consumer / dependent** | The engine or another extension that resolves and calls a point. |
| **Dependency** | A declared need resolved before initialization. A point dependency needs a provider; an extension dependency names one extension. |
| **Broker** | The host component that registers extensions, resolves dependencies, initializes them, and shuts them down. |
| **Adapter** | Generated code that makes an out-of-process gRPC provider look like the same in-process Go interface to consumers. |
| **Engine** | The host daemon, which consumes points, provides callbacks, routes calls, and publishes opted-in services on its socket. |
| **Legacy plugin** | The older Moby plugin system being replaced, or containerd plugins when discussed as prior art. The new unit is an extension. |

## Start here

- The principles and glossary above provide the short normative rules and definitions.
- [DESIGN.md](./docs/DESIGN.md) - current behavior, constraints, wire protocol, and discovery security.
- [AUTHORING.md](./docs/AUTHORING.md) - procedures, commands, code, and checklists.
