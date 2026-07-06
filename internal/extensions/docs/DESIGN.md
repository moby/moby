# Extensions — Design

This document describes the rules behind the extension model.
For terms, see [GLOSSARY.md](./GLOSSARY.md).
For the main rules, see [PRINCIPLES.md](./PRINCIPLES.md).
For authoring steps, see [AUTHORING.md](./AUTHORING.md).
For examples, see [EXAMPLES.md](./EXAMPLES.md).

This describes current behavior.
Runtime reload and scoped dependency resolvers are future work in [ROADMAP.md](./ROADMAP.md).

## Requirements

- An extension declares its id, the points it implements, and its dependencies.
  The declaration is the contract.
  The engine learns what the extension provides before any provider is called.
  For an in-process extension, this is just a value read by the host.
  No point is called and no engine flow is wired until the declaration is checked and accepted.
- Extension configuration is keyed by extension id.
  An in-process extension receives it during init.
- An extension id and a point id are different things.
  The extension is the deployed unit.
  The point is an interface contract.
  For example, the `com.docker.compose.v1` extension defines a separate API point named `com.docker.compose.api.v1`.
  This keeps dependencies on an extension separate from dependencies on a point.
- An extension id is a versioned reverse-DNS name.
  It has at least two lowercase dot-separated name segments plus a required version segment, such as `org.example.no-privileged.v1` or `com.docker.compose.v1`.
  Segments are alphanumeric and may contain hyphens, but cannot start or end with a hyphen.
  The final segment is a version such as `v0` or `v12`.
  This shape is enforced because the id is also a config key, dependency name, and named-lookup selector.
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
- Dependencies are callable during init.
  An extension calls the resolved provider directly.
- The lifecycle is register, resolve, init, run, and shutdown.
  Errors are reported per extension.
  The extension set is fixed at daemon start.
  Runtime reload, unload, health checks, and restart are future work.

## Identifiers

Three names often appear together, but they are not interchangeable.

- **Extension id** — the deployed unit's reverse-DNS name, such as `com.docker.compose.v1`.
  It is used as the config key, dependency name, and named-lookup selector.
  It is never the interface name.
- **Point id** — a versioned interface contract, such as `com.docker.compose.api.v1`.
  Providers implement it, and point dependencies name it.
  It is separate from the extension id.
- **Docker CLI / API route** — the client-facing surface, such as `docker compose up` or a REST path.
  It has its own naming and is not generated by the extension framework.

## Assumptions

- A point is a Go interface plus message types.
  The point id names the contract.
- The framework is built in-tree first.
