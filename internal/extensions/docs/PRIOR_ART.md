# Prior art — alternatives considered

The extension model is inspired by containerd plugins and HashiCorp go-plugin.
It does not adopt either one directly.
This document explains that choice.

In short, containerd/plugin covers only an in-process registry, and go-plugin covers only the process boundary.
Moby needs one model that works for both.
The same typed contract must work when a provider is compiled into the daemon or runs as a separate binary.

## What we took from containerd's plugins

These containerd ideas fit this design and are used here:

- versioned, namespaced point ids, similar to containerd's `Type`;
- dependency-ordered initialization;
- per-unit configuration delivered during init.

The principle is: broker plus dependency injection like containerd, but with dependencies that may be out of process.

## Scope: the registry is not the system

`github.com/containerd/plugin` is an in-process registry.
It has registrations, dependency ordering, and init-time lookup.
It does not launch processes.
It does not define a handshake.
It does not provide transport, proto generation, adapters, socket exposure, discovery, or a security model.

For this system, that package would cover only the broker.
The launcher, SDK, generator, and proxy would still have to be built.

containerd itself shows the gap.
For out-of-process extensibility, containerd uses several separate mechanisms: proxy plugins, NRI, transfer service pieces, runtime shims, and other hand-wired paths.
Those solve one capability at a time.
Moby already has that kind of spread in its legacy plugin systems.
The extension framework is meant to replace it with one declaration, handshake, code generation, and routing path.

## Model differences

Even where containerd/plugin overlaps, its semantics do not match this design.
Those choices make sense for containerd, but they are not the rules Moby needs here.

- **The unit is different.**
  In containerd/plugin, a registration is one type, one id, and one init function.
  In this model, the unit is the extension: one binary, one config, one lifecycle, and possibly many points.
  That matters most across the process boundary, where one process is one extension.
- **Dependencies are only by type.**
  containerd/plugin cannot express a dependency on one named extension.
- **Missing dependencies and cycles are not strict enough.**
  The extension broker fails fast on both.
  This is important when a missing provider might be a security policy.
- **Fan-out order is not a contract.**
  containerd returns providers from a map.
  This model also says fan-out order is not a contract, but points that need order must define it explicitly.
- **Loading behavior differs.**
  containerd can start degraded and report broken plugins.
  Moby extension veto points often need fail-closed behavior.
  A policy extension that fails to load should fail the daemon, not silently disappear.
- **Shutdown is part of the model.**
  The extension lifecycle includes shutdown and partial-init cleanup.
- **Resolution is typed and can happen at use time.**
  `Point[T]` keeps providers typed.
  Consumers can resolve providers when handling a request, not only during init.

Adopting containerd/plugin would require adapters around these differences.
Those adapters would become larger and more important than the package they replaced.

## Why not improve containerd/plugin upstream

Changing containerd/plugin to match this model would be an API redesign.
It would invert important containerd choices, such as fail-open loading.
Containerd would have to carry that compatibility even if it does not want the new behavior.

Even a successful upstream redesign would still cover only the in-process registry.
The transport and process-boundary pieces would remain outside that package.

This framework is also still under `internal/` and `.v0` contracts.
That lets Moby change the design quickly while it is being proven.
Moving the work through another project's review and compatibility rules would freeze the design too early.

## Why not a generic external-process SDK

A generic subprocess SDK solves the easy half: launch a process and talk to it.
The hard and ongoing work is the daemon contract.
For example: what data a create hook sees, how vetoes fail closed, how config comes from `daemon.json`, how rootless discovery works, and how services are exposed on `docker.sock`.
Those decisions belong in this tree.

A generic SDK would also need a declaration from each binary.
Once one binary can provide many points, it needs a unit for config, lifecycle, and conflicts.
That unit is the extension declaration.
The SDK would end up converging on this model, just in another repository.

HashiCorp go-plugin is the mature version of a subprocess SDK.
It was considered, but it only covers the transport side.
It does not provide in-process mode behind the same abstraction, a dependency graph, Moby lifecycle rules, config, discovery, or socket exposure.
Using it would also bring its conventions, such as magic cookies and connection-per-plugin behavior, for a relatively small amount of code.

## The wider landscape

To be an alternative, a system must support the same contract whether a provider is compiled in or runs out of process.
The surveyed systems each cover only part of that requirement.

- **hashicorp/go-plugin** is a strong transport option.
  It supports gRPC subprocess plugins, a stdout handshake, protocol negotiation, and more.
  It does not provide the unified in-process and out-of-process model, dependency graph, lifecycle, config model, discovery, or socket exposure.
- **Go's `plugin` package / dlopen** is not suitable.
  It is Linux-only, uses cgo, requires exact host toolchain and dependency compatibility, cannot unload, and does not fit static daemon builds.
- **Spec plus socket systems such as CSI, CRI, CDI, and legacy Moby plugins** solve one capability at a time.
  Each has its own spec, socket, registration, config, and lifecycle.
  That sprawl is what this framework is meant to avoid.
- **WASM hosts and OPA/Rego policy engines** provide sandboxing and hot reload for pure-compute points.
  They do not fit extensions that need mounts, netlink, subprocesses, existing binaries, or other daemon-level capabilities.
  A WASM or OPA host can still be an extension that implements a policy point.
- **In-process module registries such as Caddy modules or Traefik Yaegi plugins** do not solve the process boundary.
  Yaegi is also Go-only and interpreted.

Domain-specific systems such as CNI, NRI, and Kubernetes admission webhooks are useful prior art, but not replacements.
Each solves one domain.
A domain-specific bridge could still plug in as one extension without replacing the model.

The survey shows the same pattern throughout.
Transport SDKs lack the model.
Registries lack the process boundary.
Per-capability specs lack unification.
WASM lacks the full daemon capability surface.
Nothing existing provides one typed contract and one dependency graph for both process placements.

Two ideas are worth borrowing later.
go-plugin's handshake hardening can improve subprocess launch.
Admission webhooks' explicit failure policy is the same fail-open or fail-closed choice each point must make.

## Sequencing, not rejection

This choice does not block future convergence.
The wire protocol is small and not deeply Moby-specific.
It is a JSON startup blob on stdin, one readiness line on stdout, one Unix socket, and `Describe` / `Initialize` calls.

If the model proves useful, it can be extracted or offered upstream later.
Starting narrow and in-tree first lets the design earn production experience before becoming a public contract.
