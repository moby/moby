# Extensions — Examples

These examples show the model from [GLOSSARY.md](./GLOSSARY.md).
Each one names a point, an extension that implements it, and how the pieces are wired.

## 1. Container creation metadata (subscription / fan-out)

An extension enforces an organizational labeling convention on new containers.
It subscribes to `org.mobyproject.extension.container.create_hook.v1`.
The engine calls that point during container creation.

This is a fan-out point.
Every subscriber is called, and no subscriber order is promised.

- **Point:** `org.mobyproject.extension.container.create_hook.v1`
- **Extension:** `org.example.label-policy.v1`
- **Subscribes to:** the create_hook point
- **Depends on:** nothing

Because order is not promised, the point contract defines phases instead of relying on extension order:

- `Update(req)` — providers may adjust the request, such as adding a default label or env var.
  All `Update` calls finish first.
- `Validate(req)` — providers inspect the adjusted request and may reject the create by returning an error, such as when a required label is still missing.

`label-policy` implements both phases: `Update` fills in a default team label, and `Validate` rejects a create that still has none.
A different extension might implement only one phase.
They do not depend on each other's order.
The phase boundary provides the ordering that matters.

This hook is not a security boundary.
It sees only the subset of the create request the point models — not the full host config or OCI spec — so a check here can be side-stepped by fields it does not see, such as `--pid=host` or `--device`.
A policy that must be unbypassable belongs on the create-spec hook (`extpoints/createspec/v0`), which runs on the fully-formed OCI runtime spec — the altitude the NRI bridge and any security-hardening extension work at.

## 2. Volume driver (named selection)

The legacy volume-driver plugin becomes a point.
An extension implements `org.mobyproject.extension.volume.driver.v1`.
Its provider has no separate id.
The extension id is the identity.

`docker volume create --driver org.example.s3-volume.v1` resolves the provider from that extension.
There is no ambiguity among many drivers because the caller names the extension.
An unknown extension id fails the operation.
A short name such as `s3` could be a CLI alias for the full extension id.

- **Point:** `org.mobyproject.extension.volume.driver.v1`
- **Extension:** `org.example.s3-volume.v1`

The volume subsystem resolves the driver through the resolver and calls it.

## 3. An extension that depends on another

A backup extension needs volume-driver support.
It declares a point dependency on `volume.driver.v1`.
The broker checks that at least one provider exists before the backup extension initializes.
The backup extension can then use the resolver to get one driver, all drivers, or a configured driver by extension id.

- **Extension:** `org.example.backup.v1`
- **Depends on point:** `org.mobyproject.extension.volume.driver.v1`

The second dependency kind is an extension dependency.
For example, backup can depend on `org.example.s3-volume.v1`.
That guarantees the s3-volume extension is loaded and initialized first.
Backup may call it through its points, or only require that it exists.

Use a point dependency when the capability must exist and the provider is chosen at use time.
Use an extension dependency when a specific extension is required.

Resolution is topological.
The volume driver initializes before backup.
A missing required dependency fails backup quickly.
An optional dependency may be absent, and the extension must handle that.

## 4. Same extension, in-process vs out-of-process

The `create_hook.v1` point is a Go interface.
A `.proto` file is generated from it for the wire.
The `label-policy` extension can run in either form:

- **Compiled in:** it implements the Go interface directly.
  Calls are normal Go method calls with no marshaling.
  The extension is registered as a value and configured by id.
- **Separate binary:** it implements the generated proto service in Go or another language.
  The binary is named after its extension id and placed in the extensions directory.
  The daemon launches it, sends config through the startup handshake, speaks gRPC to it, and uses a generated adapter so callers still see the Go interface.

Consumers do not change.
In-process providers do not change.
The placement and language are packaging choices.

## 5. Docker Compose as an extension

Most Compose orchestration can move into the daemon as an extension.
This uses several parts of the model: a custom point, socket exposure, and dependencies on engine points.

- **Extension:** `com.docker.compose.v1`.
  Docker-owned extensions use `com.docker.*`, not the `org.mobyproject.*` namespace reserved for engine points.
- **Defines:** `com.docker.compose.api.v1`.
  This is a separate point from the extension id, so point dependencies and extension dependencies stay clear.
- **Opts into socket exposure:** it implements `org.mobyproject.extension.service.grpc.v0` and names the gRPC service it serves.
  The service name is `com.docker.compose.api.v1.Compose`, not the bare point id.
  The daemon publishes that service on `docker.sock` and routes `/com.docker.compose.api.v1.Compose/*` calls to Compose.
  `docker compose up` can then be a thin client call into the daemon.
  Point id, proto package, gRPC service name, and CLI verb are different identifiers.
- **Depends on points:** `container.lifecycle.v1`, `network.manager.v1`, `volume.manager.v1`, `image.v1`, and `build.v1`.
  These must be engine-provided points.

One provider can serve several caller types.
The same `com.docker.compose.api.v1` provider can answer the CLI on the socket and other extensions that depend on it.
Compose is also a consumer because it drives container, network, volume, image, and build points.

This example works well because Compose is mostly a control plane.
It does not need to pass file descriptors, network namespaces, or mounts through the extension framework.
It can hold project state and reconcile that state during its run phase.

## 6. Extension conflicts

`org.example.logger-ng.v1` and `org.example.logger.v1` cannot run together because they both claim the same log sockets.
`logger-ng` lists `org.example.logger.v1` in its conflicts.
The broker refuses to load both.

A conflict is mutual.
Declaring it on either side is enough.
It applies to the whole extension, regardless of which points the extensions implement.

A conflict is not replacement.
The broker rejects the set and reports the reason.
The operator removes one of the conflicting extensions.
The check happens at load time, so a package install that introduces a conflict fails cleanly instead of half-loading.

## 7. Live status in `docker info`

An extension can show live state in `docker info`.
It implements `org.mobyproject.extension.info.v1`.
The engine queries that point when `docker info` runs and groups the output by extension id.
The output shows current state, not static config.

- **Point:** `org.mobyproject.extension.info.v1`
- **Extension:** any extension that opts in by implementing the point

The point returns a fixed envelope with free-form content.
The CLI renders every extension the same way, while each extension chooses what to report.

```go
type Info struct {
	Summary string  // shown next to the id, e.g. "enabled, 3 plugins"
	Fields  []Field // ordered detail lines beneath it
}

type Field struct{ Name, Value string }
```

The point is opt-in and read-only.
It is for diagnostics, not control.
Because the query is live and may call an out-of-process extension, each call uses a short timeout.
A slow or failing extension appears as unavailable instead of delaying or breaking the rest of `docker info`.

## Standard points

Standard points are defined by the engine.
Capabilities the engine offers to extensions are points, not special built-ins.

**Implemented by an extension** to opt into engine behavior:

- `org.mobyproject.extension.service.grpc.v0` — publish this extension's own gRPC services on `docker.sock`.
- `org.mobyproject.extension.info.v1` — provide live status shown per extension in `docker info`.

**Provided by the engine** for extensions to depend on:

- `org.mobyproject.extension.container.lifecycle.v1`
- `org.mobyproject.extension.network.manager.v1`
- `org.mobyproject.extension.volume.manager.v1`
- `org.mobyproject.extension.image.v1`
- `org.mobyproject.extension.build.v1`

All standard points live under `org.mobyproject.*`.
Vendor extensions and vendor-defined points use their own reverse-DNS namespace, such as `com.docker.*`.
