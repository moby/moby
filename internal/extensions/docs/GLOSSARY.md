# Extensions — Glossary

These terms are used across the extension docs.

### Extension point

An **extension point** or **point** is a versioned, namespaced interface that extensions implement.
For example: `org.mobyproject.extension.volume.driver.v1`.

A point is defined by the engine or by an extension.
Its source is a Go interface and Go message types.
The `.proto` file used across the process boundary is generated from that Go contract.

A breaking change creates a new point version.

A **`.v0` point is experimental**.
It can change or be removed without backward compatibility.
An extension that implements a `.v0` point must be updated together with that point.

A point is promoted to `.v1` when its shape is stable.
From `.v1` on, breaking changes must use a new version instead of changing the point in place.

### Extension

An **extension** is the deployable unit that extends the engine.
It can be built into the daemon or run as a separate binary.
It registers one or more providers and declares its dependencies.
It is registered at daemon startup and shut down when the daemon stops.

### Provider

A **provider** is an extension's implementation of one point.
It has no id of its own.
It is identified by the extension id.

An extension can implement many points, but it implements each point at most once.
Named selection chooses a provider by extension id.

### Consumer / dependent

A **consumer** or **dependent** is anything that resolves and calls a point.
This can be the engine or another extension.

### Dependency

A **dependency** is a declared need.
The broker resolves dependencies in topological order.

A dependency on a **point** means at least one provider for that point must exist before the extension initializes.
The consumer can then ask the resolver for one provider, all providers, or a provider from a named extension.

A dependency on an **extension** names one specific extension.
That extension is initialized first.
The dependent may call it through the points it provides, or only require that it exists.

### Broker

The **broker** is the engine component that manages extensions.
It registers extensions, resolves dependencies, initializes them, and shuts them down.

### Adapter

An **adapter** makes an out-of-process provider reached over gRPC look like the same in-process Go interface.
Consumers do not need to handle transport details.

### Engine

The **engine** is the host daemon being extended.
It can be a consumer, a provider for callbacks, and the gateway that routes calls to providers.
It also publishes opted-in extension gRPC services on its socket.

> **A note on "plugin".**
> These docs avoid using *plugin* for the new model.
> Here, *plugin* only means the legacy Moby plugin system this replaces, or containerd plugins discussed as prior art.
> The new unit is always an **extension**.
