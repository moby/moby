# Extensions

Moby extensions are one model for adding engine behavior.
They replace the older per-feature plugin systems for network, volume, and log drivers.

## Mental model

- A **point** is a versioned, namespaced Go interface and its message types.
- An **extension** implements one or more points.
  It can be compiled into the daemon or run as a separate binary; the point and its callers do not change.
- The host registers extensions, resolves typed dependencies, initializes them in dependency order, and routes calls through points.
- Wire code is generated from the Go contract.
  The generated proto and adapters let a separate-binary provider be written in another language.
- Socket exposure is an explicit point: an extension may add its own gRPC services to `docker.sock` without adding a built-in REST API.

## Principles

- **Everything is a point.**
  Hooks are points called during engine flows, and extension-to-extension calls use the same provider and routing model.
- **Placement is transparent.**
  The same point contract works in-process or in a separate process; the runtime chooses direct Go or gRPC transport.
- **Dependencies are typed and explicit.**
  The broker resolves declared point or extension dependencies before initialization; importing a package does not activate it.
- **Fan-out order is not a contract.**
  A point that needs ordering defines its own phases or ordering rules.
- **Socket exposure is opt-in.**
  Exposed services must enforce their own access control.
- **Legacy plugins are replaced.**
  Network, volume, and log drivers become extension points rather than separate plugin systems.

## Glossary

| Term | Definition |
|---|---|
| **Point** | A versioned, namespaced Go interface and message types implemented by extensions. A breaking change gets a new point version; `.v0` is experimental. |
| **Extension** | A deployable unit, built into the daemon or run as a separate binary, that declares providers, dependencies, and lifecycle. |
| **Provider** | An extension's implementation of one point. It has no separate id and is identified by its extension id; an extension implements a point at most once. |
| **Consumer / dependent** | The engine or another extension that resolves and calls a point. |
| **Dependency** | A declared need resolved before initialization. A point dependency needs a provider; an extension dependency names one extension. |
| **Broker** | The host component that registers extensions, resolves dependencies, initializes them, and shuts them down. |
| **Adapter** | Generated code that makes an out-of-process gRPC provider look like the same in-process Go interface to consumers. |
| **Engine** | The host daemon, which consumes points, provides callbacks, routes calls, and publishes opted-in services on its socket. |
| **Legacy plugin** | The older Moby plugin system being replaced, or containerd plugins when discussed as prior art. The new unit is an extension. |

## Start here

- The principles and glossary above provide the short normative rules and definitions.
- [DESIGN.md](./DESIGN.md) - current behavior, constraints, wire protocol, and discovery security.
- [AUTHORING.md](./AUTHORING.md) - procedures, commands, code, and checklists.
- [ROADMAP.md](./ROADMAP.md) - deliberately deferred capabilities.
