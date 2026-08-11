# Extensions — Design

This document is the authoritative description of current behavior and constraints.
See the [principles](./README.md#principles), [glossary](./README.md#glossary), and procedural [authoring guide](./AUTHORING.md).
Runtime reload, out-of-process health handling, and scoped dependency resolvers are not current behavior.

## Contract and startup

An extension declaration contains its id, points, dependencies, conflicts, and optional `Init` and `Shutdown` functions.
The host checks it before calling a provider or wiring an engine flow.

- An in-process declaration is a value read by the host.
- For a separate binary, the daemon must launch the process before obtaining its declaration through the startup `Describe` handshake.
  A rejected declaration cannot provide a point or participate in an engine flow.
- Configuration is keyed by extension id.
  In-process extensions receive it during `Init`; separate binaries receive it in the startup handshake.
  The id is also the binary name, so the daemon can select configuration before launch.

Registration is explicit: importing a package does nothing, and there is no package-level `func init()` registry.
The host chooses the active set, which is fixed for the daemon lifetime.

## Identifiers

These names are distinct:

- **Extension id** is the deployed unit's versioned reverse-DNS name, for example `com.docker.compose.v1`.
  It has at least two lowercase, dot-separated name segments and ends in a required `v0` or `v12`-style version.
  Segments are alphanumeric with optional internal hyphens; they cannot start or end with a hyphen or contain path separators, `..`, uppercase, or other path- or shell-hostile characters.
  The id is the binary name, configuration key, dependency name, and named-lookup selector, not an interface name.
  Registration rejects invalid ids.
  Its version is a namespace element, not a semantic version, and is independent of point versions.
  Thus `com.foo.v1` and `com.foo.v2` are different extensions, binaries, and configurations that can coexist.
- **Point id** identifies a versioned interface contract, for example `com.docker.compose.api.v1`.
  It is a lowercase, dot-separated, reverse-DNS-style name ending in `vN`; segments may contain digits, hyphens, and underscores.
  Providers implement points, and point dependencies name their ids.
- **Proto package / gRPC service** is generated from a point.
  The point id becomes the `.proto` package, and each service is named `<point-id>.<Service>`.
  For example, point `org.mobyproject.extension.container.create_hook.v0` generates service `org.mobyproject.extension.container.create_hook.v0.ContainerCreateHook` and wire methods of the form `/pkg.Service/Method`.
  Socket routing uses the full service name, never the bare point id.
- **CLI / API route** is a separate client-facing name, such as `docker compose up` or a REST path; the framework does not generate it.

Publishing a point id means publishing the gRPC services in its proto package.
Socket exposure takes service names, not point ids.

## Resolution, ordering, and lifecycle

Providers are selected when a point is used:

- Named lookup fails if that extension does not provide the point.
- Single-provider lookup fails unless exactly one effective provider exists.
- All-provider lookup returns every provider.
  An extension implements a point at most once, but many extensions may implement the same point.
- Fan-out order is undefined, even if an implementation appears to preserve registration order.
  A point that needs order must define it, preferably as order-independent phases such as `Update` then `Validate`.
- Blocking, fire-and-forget, and veto semantics belong to the point contract.

`DefineSinglePoint` makes cardinality part of the point contract.
Its generated `ClientPoint` carries that fact, so the host rejects two installed providers for the point at startup.
For a single-provider point, a built-in is used only when no installed extension provides the point; an installed provider replaces it without a disable declaration.
Runtime fallback and failure behavior belongs to each point's call helper and contract.
Fan-out accessors include built-ins and installed providers; by-id lookup always honors the named extension.

Sole ownership is not a general provider setting.
A fan-out point accepts any number of providers, and a consumer may request one at use time.
Socket-exposed services have a different rule: each service name is one public address, so the host rejects a second provider for that name.

Conflicts are between extensions, not points.
Either extension listing the other is sufficient; the broker rejects the whole incompatible set and reports the reason instead of choosing a winner or substituting another extension.

Dependencies are resolved topologically before the dependent initializes.
Missing required dependencies and cycles fail fast; optional dependencies may be absent.

- A **point dependency** requires at least one provider.
  In-process callers may later use ordinary point lookup for one, all, or a provider by id.
- An **extension dependency** names one extension, which initializes first.
  The dependent may rely only on its presence or call it through its points.

An extension dependency adds naming and ordering, not another callable surface: all calls still use points.
During in-process `Init`, the provider is called directly.
During out-of-process `Init`, the extension calls a daemon callback channel that routes to the provider.
The callback currently serves one provider per dependency point; cross-process `All` and by-id selection are deferred.

The lifecycle is register, resolve, initialize, run, and shut down.
Shutdown runs in reverse dependency order and tears down only initialized extensions.
A launch or initialization failure unwinds started processes and initialized extensions; loading is all-or-nothing, not degraded.
Errors are attributed to the extension that produced them.

## Socket exposure

An extension is reachable only through daemon-internal calls unless it provides `org.mobyproject.extension.service.grpc.v0`.
This point takes a `grpc.ServiceRegistrar`, so it is resolved locally rather than sent over the wire.
It publishes an extension's own gRPC services on `docker.sock` without requiring the daemon to import that extension's proto.

- In-process, the daemon registers the services on its gRPC server beside built-ins such as BuildKit.
- Out-of-process, the SDK registers them on the extension server and reports their fully qualified names.
  The daemon proxies matching calls by name and forwards the raw gRPC stream, including metadata and status.

Service names are captured from registration, not listed manually.
The extension must declare the service-exposure point as a provider before publishing its service inventory.
A name cannot collide with another extension or a service already served by the daemon; collision fails startup rather than shadowing the existing service.

These services use the raw gRPC endpoint beside the daemon's own gRPC services.
Authorization plugins gate the REST API, not this endpoint; an exposed service must enforce any access control it needs.

## Separate-process protocol

The binary name is its extension id (`<id>.exe` on Windows), and the daemon launches it from the configured extension directory.
Startup proceeds as follows:

1. The daemon writes JSON to stdin with `endpoint`, `protocolVersion`, `config`, and, when dependencies are offered, `callbackEndpoint`.
   The current protocol version is `1`; `config` is the parsed `daemon.json` entry for the id.
2. The SDK listens on the supplied Unix socket, then writes exactly `ready\n` to stdout.
   Stdout is reserved for this line: any earlier output corrupts the handshake and fails launch.
   The daemon also drains later stdout so it cannot block the process.
3. The daemon captures stderr in its logs, dials the Unix socket, and calls the runtime `Describe` RPC.
   The returned id must match the binary name.
   Before registration, the daemon validates the declaration, including provider points, dependencies, conflicts, and registered service names.
4. The broker calls the runtime `Initialize` RPC in dependency order.
   The SDK runs the extension's `Init` then and its `Shutdown` during shutdown.
   A dependency callback socket is available at initialization when the host offered the declared dependency points.
   The SDK connects lazily, and `Depends` supplies generated client adapters.

Point calls use generated gRPC server and client wiring.
The generated client adapter presents the same Go interface to the host.
A dead process produces gRPC errors until the daemon restarts.
There is no watchdog, reconnect loop, or restart policy.

## Wire contract and compatibility

The Go interface and message structs are the source of truth.
`mobyextgen` generates the proto, protobuf message code, gRPC service, client and server points, adapters, and conversions without `protoc`, plugins, or other `PATH` tools.
In-process providers implement Go directly; out-of-process providers use gRPC.

Messages use proto3.
Supported fields are scalar values, `string`, `[]byte`, repeated scalars such as `[]string`, string-keyed scalar maps such as `map[string]string`, pointer nested messages such as `*Other`, and repeated nested messages such as `[]Other`.
A single nested message must be a pointer; value nested messages are rejected.
Other shapes fail generation rather than producing ambiguous wire code.

Proto3 scalars have no presence: `""`, `0`, and `false` are indistinguishable from unset.
To distinguish "leave unchanged" from an explicit zero, use a message wrapper such as `*Thing`, a separate state flag, or an interpreted `bytes` payload.
Optional scalars, `oneof`, enums, and well-known types such as timestamp and duration are unsupported.
Use a Unix `int64` time, documented string enum-like values, or a `bytes` / JSON payload instead.
There is no typed error schema: Go `error` crosses gRPC as a status, and each point documents its meaning, such as a veto.

Within `.v1`, add fields with new numbers; never renumber or reuse a number, or change a field's type.
Old peers ignore added fields, while new peers see zero values when talking to old peers.
Deleting a field burns its number: record it with `mobyextgen:reserved` so the generator emits `reserved N;` and rejects reuse.
Breaking changes require a new `.vN`; `.v0` remains experimental.

## Discovery security

Discovery is a root-code-execution boundary.
The daemon scans the extensions directory and launches every accepted executable, often as root; each binary is trusted daemon code.

- `--extension-dir`, or the default `/usr/libexec/docker/moby-extensions`, is trusted.
  Treat it as a root-owned program directory: only package managers or operators should install files there, and unprivileged users must not be able to write to it.
- A world-writable directory or binary is skipped with a warning.
  A binary or directory owned by anyone other than root or the daemon user is also skipped because its owner could rewrite daemon-executed code.
  Only executable files with valid extension-id names are launched; stray tools and build leftovers are not executed.
- These checks are backstops, not the complete trust model.
  Group-writable binaries are **not** refused; only the world-writable (`o+w`) bit is checked.
  An untrusted group could therefore rewrite a group-writable binary and gain daemon code execution.
  Keep the directory and binaries writable only by root or the daemon user, and do not grant their group to untrusted users.
- Symlink targets are checked as ordinary files.
  Where ownership cannot be determined from file metadata, notably on Windows, the owner check is not enforced; ACL and group policy remain the operator's responsibility.
- One accepted extension that cannot launch, describe, initialize, or coexist with the set fails daemon startup.
  Because the default directory is scanned automatically, a broken or incompatible binary there blocks startup until removed.
- In rootless mode, the daemon and libexec directory belong to the user, so "trusted" means trusted by that user; the world-writable check still applies.
  Packaging should install one non-world-writable binary per extension, named after its id.

There is no sandbox or separate permission model after acceptance.
An extension can do anything the daemon can do.
Administrators control this risk by deciding what to install and inspecting each declaration.

## Design rationale and tradeoffs

Existing systems cover only parts of this model.
Containerd plugins provide versioned names, dependency ordering, and init-time configuration, but remain an in-process registry without process launch, a handshake, generated transports and adapters, socket exposure, or discovery security.
Moby also needs one unit that can expose several points and depend on an out-of-process extension; adapting the registry would split registration from transport and impose different failure and lifecycle rules.
Generic process SDKs such as go-plugin provide a process boundary, gRPC, and a handshake, but not Moby's typed point graph, unified in-process mode, configuration and lifecycle rules, fail-closed loading, or `docker.sock` exposure.
Go's `plugin` and `dlopen` depend on platform, cgo, and exact toolchain or dependency compatibility, cannot be unloaded reliably, and do not suit static daemon builds.
Capability-specific systems such as CSI, CRI, CDI, CNI, NRI, and legacy Moby plugins remain useful in their domains but retain separate declarations, sockets, configuration, and lifecycles instead of one typed dependency graph.
WASM and OPA-style hosts suit sandboxed, reloadable pure computation, not mounts, netlink, subprocesses, existing binaries, or the daemon's full capability surface, though one could implement a point.

As with admission webhooks, each point must explicitly choose fail-open or fail-closed behavior.
Security and veto points generally fail closed; transport or diagnostic points may choose otherwise.
The framework is built in-tree, with external-binary discovery and packaging on the same extension-facing Go API.
Transport is a host assembly choice: direct Go and gRPC use the same point interface.
Keeping the startup protocol small and the framework in-tree leaves room for extraction or convergence after production experience instead of freezing an unproven public contract.
