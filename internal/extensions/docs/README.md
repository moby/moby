# Extensions

Moby extensions are a single model for adding engine behavior.
They replace the older per-feature plugin systems for things like network, volume, and log drivers.

An extension point is a Go interface.
An extension implements one or more points and is registered with the engine at startup.

The Go interface and its message types are the contract.

## Table of contents

- [GLOSSARY.md](./GLOSSARY.md) — names for the main concepts: point, extension, provider, consumer, dependency, broker, and engine.
- [PRINCIPLES.md](./PRINCIPLES.md) — the rules this model follows.
- [DESIGN.md](./DESIGN.md) — the detailed rules for resolution and identifiers.
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
