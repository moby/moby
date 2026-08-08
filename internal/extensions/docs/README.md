# Extensions

Moby extensions are a single model for adding engine behavior.
They replace the older per-feature plugin systems for things like network, volume, and log drivers.

An extension point is a Go interface.
The same point works whether the extension is compiled into the daemon or runs as a separate binary.
The engine chooses where the extension runs; the extension code should not care.

The Go interface and its message types are the contract.
The `.proto` file is generated from that contract so separate-binary extensions can be written in other languages.

## Table of contents

- [GLOSSARY.md](./GLOSSARY.md) — names for the main concepts: point, extension, provider, consumer, dependency, broker, adapter, and engine.
- [PRINCIPLES.md](./PRINCIPLES.md) — the rules this model follows.
- [DESIGN.md](./DESIGN.md) — the detailed rules for resolution, identifiers, and discovery security.
- [AUTHORING.md](./AUTHORING.md) — how to add a point or write an extension.
- [EXAMPLES.md](./EXAMPLES.md) — examples, standard points, and the planned info point.
- [PRIOR_ART.md](./PRIOR_ART.md) — why this is not containerd/plugin, go-plugin, or another existing plugin model.
- [ROADMAP.md](./ROADMAP.md) — work that is intentionally left for later.

## Use cases

- **Engine hook** — the engine calls an extension during a flow.
  For example, an extension can inject default labels or env when a container is created, or a bridge can reshape — or, as an unbypassable veto, reject — the OCI runtime spec before start.
  Enforcement that must hold belongs on the spec-level hook, which sees the fully-formed runtime spec, not the create-time hook, which sees only a partial view of the request.
- **Driver** — a network, volume, or log driver implemented as an extension point instead of a legacy plugin.
  The caller selects it by extension id.
- **Own API on the socket** — an extension publishes its own gRPC service on `docker.sock` for external clients.
  The daemon proxies the service without knowing its proto.
- **Compiled-in or separate binary** — the same point can be built into the daemon or shipped as a standalone binary in any language.
