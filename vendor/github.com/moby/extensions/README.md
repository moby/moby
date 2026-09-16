# Extensions

> [!WARNING]
> ⚠️ This project is a work in progress.

Moby extensions are one model for adding engine behavior.
They replace the older per-feature plugin systems for network, volume, and log drivers.

## Mental model

- A **Point** is a versioned, namespaced Go interface and its message types.
- A daemon- or Host-owned capability is an ordinary Point too. Its in-process
  implementation is registered through an extension `Declaration`, rather than
  through a separate hook or plugin mechanism.
- An extension declares the Points it provides and the typed dependencies it
  needs. A consumer declares `Point.Dependency()` and resolves the typed Go
  interface during `Declaration.Init`.
- A Point contract is transport-neutral. The same interface and messages work
  for a direct in-process call or for a launched process over gRPC.
- Placement is assembled with generated wiring: `ServerPoint` serves an
  ordinary Point and `ClientPoint` presents the same typed interface over a
  launched-provider or dependency-callback connection.
- The Host registers the fixed extension set, initializes it in dependency order,
  and shuts it down in reverse dependency order. Importing a package does not
  activate an extension.

## Principles

- **Everything is a Point.**
  Host capabilities, engine hooks, and extension-to-extension calls use
  ordinary Point contracts and the same provider and resolution model.
- **Dependencies are typed and explicit.**
  `Point.Dependency()` controls presence and initialization order;
  `Point.Single`, `Point.All`, and `Point.ByExtension` perform typed resolution.
  A dependency declaration does not inject a field automatically.
- **Contracts do not choose placement.**
  Direct Go and gRPC are two ways to place the same Point. Generated
  `ServerPoint` and `ClientPoint` wiring makes that choice without changing the
  contract.
- **The Host owns lifecycle.**
  Provider extensions must finish their `Init` work before returning. The Host
  then initializes dependents and later shuts them down in reverse dependency
  order.
- **No private activation paths.**
  Consumers keep Point interfaces, not concrete Host or daemon pointers, and
  the Host lifecycle replaces later manual `Activate` calls.
- **Publication is opt-in.**
  An ordinary Point is externally reachable only when it is offered, Host
  policy allows publication, and generated server wiring is supplied.
  Published services enforce their own access control.
- **Fan-out order is not a contract.**
  A Point that needs ordering defines its own phases or ordering rules.
- **Legacy plugins are replaced.**
  Network, volume, and log drivers become ordinary extension Points rather than
  separate plugin systems.

## Authoring a host capability

A Host capability is an ordinary Point owned by the daemon or application. The
contract, daemon-owned provider, and consumer can live in separate packages.
Imports are omitted from these examples. `workloadv0` is the handwritten
contract package. After generation, import its generated `protogen` package as
`workloadpb`; it provides `workloadpb.ServerPoint` and
`workloadpb.ClientPoint`.

The contract package defines one deciding Point:

```go
// Package workloadv0 defines the daemon workload runtime Point.
package workloadv0

type Runtime interface {
	Run(context.Context, *Request) (*Response, error)
}

type Request struct {
	Name string `pb:"1"`
}

type Response struct {
	Message string `pb:"1"`
}

var Point = extensions.DefineSinglePoint[Runtime](
	"org.example.daemon.workload.v0",
)
```

Keep the contract expressible by the generated wire representation. Do not put
concrete Host types, internal backend structs, function callbacks, or channels
in a Point interface or its messages.

The daemon package owns the provider and registers it through the contract
Point:

```go
type runtime struct{}

func (runtime) Run(
	_ context.Context,
	req *workloadv0.Request,
) (*workloadv0.Response, error) {
	return &workloadv0.Response{Message: "ran " + req.Name}, nil
}

var runtimeExtension extensions.Extension = extensions.New(extensions.Declaration{
	ID: "org.example.daemon-runtime.v1",
	Providers: []extensions.Provider{
		workloadv0.Point.Provide(runtime{}),
	},
})
```

The consumer package keeps the resolved interface on its stateful extension:

```go
type consumer struct {
	runtime workloadv0.Runtime
}

func (c *consumer) Declaration() extensions.Declaration {
	return extensions.Declaration{
		ID: "org.example.workload-consumer.v1",
		Dependencies: []extensions.Dependency{
			workloadv0.Point.Dependency(),
		},
		Init: func(
			_ context.Context,
			_ extensions.Config,
			resolver extensions.Resolver,
		) error {
			runtime, err := workloadv0.Point.Single(resolver)
			if err != nil {
				return err
			}
			c.runtime = runtime
			return nil
		},
	}
}
```

In-process composition uses both extensions. Retain the returned Host and call
`Shutdown` so it owns and releases lifecycle resources:

```go
h, err := host.New(ctx, host.WithExtensions(runtimeExtension, &consumer{}))
if err != nil {
	return err
}
defer h.Shutdown(context.Background())
```

For a launched consumer, the Host registers the daemon-owned provider and
serves it on the dependency callback:

```go
h, err := host.New(ctx,
	host.WithRuntimeDir(runtimeDir),
	host.WithDirs(extensionDir),
	host.WithExtensions(runtimeExtension),
	host.WithDependencyProviders(workloadpb.ServerPoint),
)
if err != nil {
	return err
}
defer h.Shutdown(context.Background())
```

The consumer process has no ordinary Point of its own, so it registers without
a `ServerPoint`, installs the generated client wiring, and listens:

```go
func run(ctx context.Context) error {
	srv := sdk.NewServer()
	if err := srv.Register(&consumer{}); err != nil {
		return err
	}
	srv.Depends(workloadpb.ClientPoint)
	return srv.Listen(ctx)
}
```

`Declaration.Dependencies`—here populated by `workloadv0.Point.Dependency()`—is
the logical dependency. `WithDependencyProviders` does not declare a dependency
or create a provider; it exposes an already registered effective provider over
the callback socket. `Depends` does not declare a dependency or create a
provider either; it installs the generated client wiring used to resolve that
declared dependency. A process that also provides ordinary Points passes every
corresponding generated `ServerPoint` to `Register`.
The dependency callback exposes one effective provider for each Point. The SDK
closes its callback connection before the launched extension's `Shutdown`, so
do not use a retained callback dependency during shutdown.

### Initialization and readiness

The extension providing a capability must establish the readiness it promises
before its `Init` returns. Because the consumer declares
`workloadv0.Point.Dependency()`, dependency ordering makes the provider's
`Init` finish before the consumer's `Init` begins. A successful `host.New`
means that all extension initializers returned successfully.

For a launched process, the `ready\n` line means only process
listener/runtime-handshake readiness: the SDK has bound its extension socket
and registered its runtime service. It does not mean that the extension's
`Init` has completed. Do not retain concrete Host or daemon pointers as
capability dependencies, and do not manually call `Activate` after Host
startup; declare a Point dependency and perform setup in `Init` instead.

## Ordinary Point publication

Publication uses the same ordinary Point contract; there is no separate service
contract. This example defines a `greeterv0` contract:

```go
// Package greeterv0 defines the published Greeter Point contract.
package greeterv0

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

`mobyextgen` infers the fully qualified service name as
`<PointID>.<InterfaceName>`, here
`org.mobyproject.extension.example.greeter.v0.Greeter`. The interface name is
part of the wire identity, so keep it stable within a Point version. After
generation, import the generated `protogen` package as `greeterpb`. It provides
`ServerPoint` to serve the ordinary Point, `ClientPoint` to construct a typed
provider over a launched connection, and `NewClient(conn)` to construct the
handwritten Go interface over a Host connection.

The extension implements the ordinary Point and separately offers it:

```go
type greeter struct{}

func (greeter) Greet(
	_ context.Context,
	req *greeterv0.HelloRequest,
) (*greeterv0.HelloReply, error) {
	return &greeterv0.HelloReply{Message: "hello " + req.Name}, nil
}

var extension = extensions.New(extensions.Declaration{
	ID: "org.example.greeter.v1",
	Providers: []extensions.Provider{
		greeterv0.Point.Provide(greeter{}),
		servicev0.Offer(greeterv0.Point),
	},
})
```

An offer can select a subset of several implemented Points:

```go
Providers: []extensions.Provider{
	foov1.Point.Provide(fooImpl{}),
	barv1.Point.Provide(barImpl{}),
	servicev0.Offer(foov1.Point),
}
```

`Offer` is eligibility, not authorization or transport registration. It does
not select gRPC or register a service. The service metadata Point carries the
offer; Host policy makes the publication decision for the service generated
from the ordinary Point.

The policy receives the Host-attested extension identity and requested Point.
For example, an in-process Host can allow this extension's ordinary Point and
its publication metadata Point:

```go
h, err := host.New(ctx,
	host.WithExtensions(extension),
	host.WithProviderPolicy(host.PointPolicyFunc(
		func(
			identity extensions.ExtensionIdentity,
			point extensions.PointID,
		) host.PointPolicyResult {
			allowedPoint := point == greeterv0.Point.ID() ||
				point == servicev0.Point.ID()
			if identity.ID == "org.example.greeter.v1" && allowedPoint {
				return host.Allow()
			}
			return host.Drop()
		},
	)),
	host.WithPointServers(greeterpb.ServerPoint),
)
if err != nil {
	return err
}
defer h.Shutdown(context.Background())
```

A nil policy preserves all internally wired providers but drops publication.
`host.Allow()` keeps the requested ordinary provider or publishes the offered
set. `host.Drop()` silently omits an ordinary provider without unloading its
extension or leaves the offered set private. `host.Reject(cause)` fails Host
construction while preserving its error cause. The zero value of
`host.PointPolicyResult` is invalid or unspecified and rejects the request. A
typed-nil `host.PointPolicyFunc(nil)` is still a policy value and rejects the
request as well. For an in-process offer, allow the ordinary Point ID as well
as `servicev0.Point.ID()` so provider admission and publication both succeed;
an offered-only process consults policy only with the service metadata Point
ID.

For a separate binary, pass every ordinary provider's generated `ServerPoint`
through the SDK:

```go
sdk.Main(extension, sdk.WithServerPoints(greeterpb.ServerPoint))
```

An in-process provider has no private SDK server, so the Host needs
`host.WithPointServers(greeterpb.ServerPoint)` to translate requests into calls
on the Go implementation. Use
`host.WithClientProviders(greeterpb.ClientPoint)` only when the Host itself
or another in-process consumer needs to call a launched provider. That is the
opposite direction from a dependency callback: `WithClientProviders` wires
Host-to-process calls, while `WithDependencyProviders` and `Depends` wire a
launched consumer's calls back to an already registered Host provider.

External callers use the generated handwritten client over the Host connection:

```go
client := greeterpb.NewClient(hostConn)
reply, err := client.Greet(ctx, &greeterv0.HelloRequest{Name: "world"})
```

The current backend is gRPC. Generated service registration records the fully
qualified service name, and the proxy forwards the raw gRPC stream, including
metadata and status. Arbitrary raw gRPC publication is not part of the
extension API.

## Glossary

| Term | Definition |
|---|---|
| **Point** | A versioned, namespaced, transport-neutral Go interface and message contract. It may be provided by an in-process or separate-process extension, including a Host capability wrapper; a breaking change gets a new Point version, and `.v0` is experimental. |
| **Host capability** | A capability owned by a daemon or application and exposed as an ordinary Point through an in-process extension declaration. |
| **Host** | The framework component embedded in or configured by a daemon/application. It registers extensions, resolves Points, assembles generated wiring, and manages lifecycle; it is not synonymous with the daemon/application. |
| **Extension** | A deployable unit that runs in-process with the Host or in a separate process, and declares providers, dependencies, conflicts, and lifecycle callbacks. |
| **Extension identity** | The logical extension ID plus the Host-attested origin (`builtin` or `executable`). The extension declares the logical ID; the Host validates it and supplies the origin. |
| **Provider** | An implementation of a Point declared by an extension. It has no separate ID, is associated with the declaring extension's Host-attested identity, and each extension may declare at most one implementation per Point. |
| **Consumer / dependent** | A daemon/application flow or extension that resolves and calls a Point. An extension dependent declares the Point or extension dependency that controls initialization order. |
| **Dependency** | A declared need for a Point provider or extension, resolved before the dependent initializes. |
| **ClientPoint** | Generated wiring for an ordinary Point. On the Host side, it turns a launched provider connection into the typed interface used by Host consumers; on the process side, it turns the dependency-callback connection into the typed interface used by a launched consumer. |
| **ServerPoint** | Generated registration that serves an ordinary Point from an SDK server, a dependency callback, or an in-process publication adapter. |
| **Offer** | Extension metadata marking selected ordinary Points as eligible for external publication; Host policy still makes the publication decision. |
| **Publication** | Host-controlled exposure of an offered Point's generated service to external callers. |
| **Dependency callback** | The Host gRPC server through which a launched consumer calls an already registered Host provider. |
| **Broker** | The Host component that registers extensions, resolves dependencies, initializes them, and shuts them down. |
| **Adapter** | Generated code that makes an out-of-process gRPC provider look like the same in-process Go interface to consumers. |
| **Engine** | The daemon or application that embeds or configures a Host and runs the flows that consume and provide Points. |
| **Legacy plugin** | The older Moby plugin system being replaced, or containerd plugins when discussed as prior art. The new unit is an extension. |

## Start here

- The mental model, principles, authoring flow, and glossary provide the short normative rules and definitions.
- [DESIGN.md](./docs/DESIGN.md) - current behavior, constraints, wire protocol, and discovery security.
- [AUTHORING.md](./docs/AUTHORING.md) - procedures, commands, code, and checklists.
