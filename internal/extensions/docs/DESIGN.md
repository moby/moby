# Extensions — Design

This document describes the rules behind the extension model.
For terms, see [GLOSSARY.md](./GLOSSARY.md).
For the main rules, see [PRINCIPLES.md](./PRINCIPLES.md).
For authoring steps, see [AUTHORING.md](./AUTHORING.md).
For examples, see [EXAMPLES.md](./EXAMPLES.md).

This describes current behavior.
Runtime reload, out-of-process health handling, and scoped dependency resolvers are future work in [ROADMAP.md](./ROADMAP.md).

## Requirements

- An extension declares its id, the points it implements, and its dependencies.
  The declaration is the contract.
  The engine learns what the extension provides before any provider is called.
  For an in-process extension, this is just a value read by the host.
  For an out-of-process extension, the daemon must start the process and read the declaration through the startup `Describe` handshake.
  That launch is unavoidable, but no point is called and no engine flow is wired until the declaration is checked and accepted.
- Extension configuration is keyed by extension id.
  An in-process extension receives it during init.
  An out-of-process extension receives it through the startup handshake.
  The daemon can find the config before launch because the binary name is the extension id.
- An extension id and a point id are different things.
  The extension is the deployed unit.
  The point is an interface contract.
  For example, the `com.docker.compose.v1` extension defines a separate API point named `com.docker.compose.api.v1`.
  This keeps dependencies on an extension separate from dependencies on a point.
- An extension id is a versioned reverse-DNS name.
  It has at least two lowercase dot-separated name segments plus a required version segment, such as `org.example.no-privileged.v1` or `com.docker.compose.v1`.
  Segments are alphanumeric and may contain hyphens, but cannot start or end with a hyphen.
  The final segment is a version such as `v0` or `v12`.
  This shape is enforced because the id is also a binary name, config key, dependency name, and named-lookup selector.
  The shape excludes path separators, `..`, uppercase letters, and shell-hostile characters.
  A bad id is rejected during registration.
  - The version segment is required, but it is only a namespace element.
    It is not a semantic version.
    It is also separate from point versions.
  - Moving from `com.foo.v1` to `com.foo.v2` creates a new extension.
    It is not an in-place upgrade.
    The two extensions can exist together during migration, and dependents can name the one they need.
- Provider lookup is exact at use time.
  Lookup by extension id fails if that extension does not provide the point.
  Single-provider lookup fails unless exactly one provider exists.
  All-provider lookup returns every provider.
- Fan-out order is undefined.
  An extension must not rely on running before or after another extension.
  If order matters, the point must define it.
  Prefer order-independent phases, such as `Update` followed by `Validate`.
  A point that truly needs total order must define that order itself.
- Call semantics belong to each point.
  A point decides whether calls block, whether they are fire-and-forget, and whether a provider can veto an operation.
- An extension implements a given point at most once.
  More than one extension can implement the same point.
  Named lookup distinguishes them by extension id.
- Conflicts are declared at the extension level.
  An extension can list other extensions it cannot run with.
  Either side declaring the conflict is enough.
  The broker rejects the incompatible set and reports the reason.
  It does not pick a winner or replace one extension with another.
- Sole ownership is not a general provider setting.
  A fan-out point can have any number of providers.
  A consumer that needs exactly one asks for a single provider at use time.
  A point exposed on the API socket is different because its service name is one public address.
  The host rejects a second provider for the same exposed service name.
- Dependencies are resolved in topological order.
  Missing required dependencies and dependency cycles fail fast.
  Optional dependencies may be absent.
- There are two dependency kinds.
  Both are resolved before the dependent initializes.
  - **Point dependency** — requires at least one provider for a point before initialization.
    The dependent chooses a provider at use time through normal lookup.
  - **Extension dependency** — requires one named extension.
    It guarantees that extension is loaded and initialized first.
    The dependent can call the points that extension provides, or only rely on its presence.
- Calls still go through points.
  An extension's callable surface is the points it implements.
  An extension dependency only scopes those points to one named extension and adds ordering.
  External access to an extension is separate and opt-in through socket exposure.
- Dependencies are callable during init.
  In-process extensions call the provider directly.
  Out-of-process extensions call back to the daemon over a callback channel, and the daemon routes the call to the real provider.
- Out-of-process providers use gRPC.
  The `.proto` file is generated from the point's Go contract.
  In-process providers implement the Go interface directly.
  For out-of-process providers, a generated client adapter presents the same Go interface and converts to and from proto messages.
- The lifecycle is register, resolve, init, run, and shutdown.
  Errors are reported per extension.
  The extension set is fixed at daemon start.
  Runtime reload, unload, health checks, and restart are future work.

## Identifiers

Four names often appear together, but they are not interchangeable.

- **Extension id** — the deployed unit's reverse-DNS name, such as `com.docker.compose.v1`.
  It is used as the binary name, config key, dependency name, and named-lookup selector.
  It is never the interface name.
- **Point id** — a versioned interface contract, such as `com.docker.compose.api.v1`.
  Providers implement it, and point dependencies name it.
  It is separate from the extension id.
- **Proto package / gRPC service name** — generated from the point.
  The point id becomes the `.proto` package.
  Each service is named `<point-id>.<Service>`.
  For example, the engine's `container.create_hook.v0` point generates `org.mobyproject.extension.container.create_hook.v0.ContainerCreateHook`.
  This full service name is what appears on the wire in `/pkg.Service/Method`.
  It is also what the socket-exposure proxy routes by.
  It is not the bare point id.
- **Docker CLI / API route** — the client-facing surface, such as `docker compose up` or a REST path.
  It has its own naming and is not generated by the extension framework.

When a doc says an extension "publishes `com.docker.compose.api.v1`", read that as shorthand for the gRPC services under that package.
The socket-exposure point takes service names, not point ids.

## Security model for discovery

Discovery is a root-code-execution boundary.
The daemon scans the extensions directory and launches every executable it accepts.
The daemon often runs as root.
A binary in that directory is therefore trusted daemon code.

The model is:

- **`--extension-dir` is trusted.**
  Configuring it, or using the default `/usr/libexec/docker/moby-extensions`, means its contents may run as the daemon.
  It should be managed like any other root-owned program directory.
  Package managers or operators should install files there.
  Unprivileged users must not be able to write to it.
- **The daemon rejects obvious local escalation.**
  A world-writable directory or world-writable binary is skipped with a warning, because those permissions would let any local user change code that the daemon runs.
  So is a binary or directory owned by a user other than root or the daemon itself, which that owner could rewrite and have run as the daemon.
  And only files whose name is a valid extension id are launched, so a stray executable that happens to share the directory (a build leftover, a helper tool) is never executed.
  These checks are a backstop, not the full trust model.
- **Broader ownership and group policy belong to the operator.**
  Beyond the owner and world-writable checks, the OS and deployment policy decide which further owners or groups are trusted.
  The daemon does not recreate that policy.
  In particular, a *group-writable* binary is **not** refused: only the world-writable (`o+w`) bit is checked, so if the file's group has write access, every member of that group can rewrite it and gain code execution as the daemon.
  Keep the extensions directory and its binaries writable only by root (or the daemon's user), and do not grant their group to untrusted users.
- **One bad extension fails daemon startup.**
  Loading is all-or-nothing: a binary that will not launch, describe, or initialize fails the whole daemon, by design, rather than starting degraded.
  Because the default directory is scanned automatically, a broken or incompatible binary dropped there will prevent the daemon from starting until it is removed.
- **Symlinks are followed as ordinary files.**
  If a symlink target is untrusted or world-writable, the same target permission checks reject it.
- **Rootless mode moves the boundary to the user.**
  The daemon runs as the user, and the extension directory lives under the user's libexec home.
  "Trusted" means trusted by that user.
  The same world-writable check still applies.
- **Packaging should install one binary per extension.**
  The binary is named after the extension id and placed in the extensions directory with non-world-writable permissions.

After an extension is accepted, it is trusted.
There is no sandbox and no separate permission model.
An extension can do anything the daemon can do.
The administrator evaluates that risk by controlling what is installed and by inspecting each extension's declaration.

## Assumptions

- A point is a Go interface plus message types.
  Its `.proto` file and wire adapters are generated from them.
  The point id names the contract.
- Transport is an engine assembly choice.
  A call can be in-process or gRPC without changing the point interface.
- The framework is built in-tree first.
  External-binary discovery and packaging are layered on without changing the extension-facing API.
