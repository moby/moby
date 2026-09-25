# Extensions

> [!WARNING]
> This project is still a work in progress.

Moby extensions provide a common model for extending engine behavior.
They are intended to replace the separate plugin systems currently used by network, volume, and logging drivers.

An extension can run inside the Host process or as a separate process.
In both cases, it implements the same Go interface.

See the [authoring guide](./docs/AUTHORING.md) for examples and setup instructions.
For details about the architecture, wire protocol, current limitations, and discovery security, see the [design guide](./docs/DESIGN.md).

## How it works

Extensions are built around Points.
A Point is an interface that describes a capability, such as running a workload.
Providers implement Points, and consumers call them.
The Host registers these components, connects consumers to providers, and manages their lifecycle.

For example, a daemon can expose its workload runtime as a Point.
An extension can then use that capability without depending on the daemon's internal runtime implementation.
The same mechanism is used for daemon-owned capabilities and communication between extensions.

To add an extension:

1. **Define the contract.**

   A Point assigns a namespaced, versioned identity to a Go interface and its message types.

2. **Declare what the extension provides and depends on.**

   Providers implement Points.
   Consumers declare their dependencies using `Point.Dependency()`.

3. **Register the extension with the Host.**

   The daemon or application decides which extensions to load and supplies any generated wiring required for cross-process calls.
   Merely importing an extension package does not activate it.

4. **Resolve dependencies during initialization.**

   In `Declaration.Init`, consumers resolve their declared dependencies using `Point.Single`, `Point.All`, or `Point.ByExtension`.
   The resolved implementation is not automatically assigned to a field.

5. **Call the Point interface.**

   Consumers retain and call the resolved interface rather than a concrete Host or daemon type.
   Calls go directly to an in-process provider or through generated gRPC code when the provider runs in another process.

The Host initializes providers before their consumers and shuts them down in the opposite order.
A provider must be ready for use when its `Init` method returns.

Keep the Host returned by `host.New` and call `Shutdown` when the application exits so that lifecycle resources are released.

Breaking changes require a new Point version.
The `.v0` suffix identifies an experimental contract.
For fan-out calls, provider order is unspecified unless the Point defines its own ordering rules.

## Running extensions

A Point defines an interface, not where its implementation runs.
In-process consumers call the Go implementation directly.
Cross-process consumers use generated gRPC wiring.

Because the same contract may cross a process boundary, Point interfaces and messages should not contain Host types, internal backend structures, callbacks, or channels.

The `mobyextgen` generator creates the required wire code:

* `ServerPoint` exposes a Point implementation over gRPC.
* `ClientPoint` presents the remote implementation through the same typed Go interface.

The exact setup depends on where the consumer and provider run:

* Register in-process extensions with `host.WithExtensions`.
* When the Host calls a provider in a launched process, configure the Host with `host.WithClientProviders` and the generated `ClientPoint`.
  The provider registers its generated `ServerPoint` with the SDK.
* When a launched consumer calls a Host provider, configure the Host with `host.WithDependencyProviders` and the generated `ServerPoint`.
  In the consumer, pass the generated `ClientPoint` to `Server.Depends`.

The last case exposes an existing Host provider to the launched process.
The consumer must still declare the dependency itself.

A dependency callback currently exposes one effective provider for each Point.
Do not retain and call that dependency during `Shutdown`, because the SDK closes the connection before shutdown begins.

For launched processes, the `ready\n` handshake only confirms that the listener is ready.
It does not mean that the extension's `Init` method has completed.

See the [authoring guide](./docs/AUTHORING.md) for complete examples covering code generation, SDK registration, dependency resolution, and deployment.

## Publishing services

Implementing a Point makes it available within the extension system, but does not automatically expose it to external callers.

Publishing a Point requires three things:

* The extension offers it using `servicev0.Offer`.
* Host policy permits it through `host.WithProviderPolicy`.
* The generated server wiring is registered.

For an in-process provider, supply the server wiring through `host.WithPointServers`.
A provider running in a separate process registers its `ServerPoint` with the SDK.

An offer is only a request to publish a Point; it does not grant access or register the transport.
If no provider policy is configured, external publication is disabled, while providers remain available to internal consumers.

Published services are responsible for enforcing their own access control.

See the [publication examples](./docs/AUTHORING.md#publishing-an-ordinary-point) for complete configurations and generated client usage.

## Glossary

### Point

A named and versioned Go interface, together with its message types.
It describes a capability independently of its implementation or location.

### Extension

A component that can provide Points, depend on other Points or extensions, and perform initialization and shutdown work.
It may run inside the Host or in a separate process.

### Host

The framework component embedded in a daemon or application.
It registers extensions, resolves their dependencies, and manages their lifecycle.

### Provider

An extension that implements a Point.

### Consumer

Code that resolves and calls a Point.
An extension may be both a consumer and a provider.

### Dependency

A declared requirement for a Point provider or another extension.
The Host uses dependencies to verify that requirements are available and to determine initialization order.

### Host capability

Functionality owned by the daemon or application and exposed as a Point through an in-process extension.

### ClientPoint / ServerPoint

Generated code used to call a Point across a process boundary.
`ClientPoint` presents the Point's Go interface to the consumer, while `ServerPoint` forwards incoming calls to the implementation.

### Offer

A request from an extension to make one or more of its Points available to external callers.

### Publication

The Host exposing an offered Point to external callers after applying its policy and registering the required server wiring.

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
