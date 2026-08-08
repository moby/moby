# Extensions — Roadmap

This work is intentionally not built yet.
The current interfaces are shaped so these features can be added later without changing how points and extensions are written.

- **Runtime reload and unload.**
  Today, the extension set is fixed when the daemon starts.
  Hot loading or removing providers would need dependency resolution while the daemon is running.
  It would also need a safe way to stop consumers from calling a removed provider.
  The first tool is resolving providers at use time.
  Broker-owned invalidatable references should only be added if that is not enough.
- **Dependency-scoped resolvers.**
  Resolution works today, but an extension's `Init` receives the whole broker as its resolver.
  It is not yet limited to the dependencies the extension declared.
  Cross-process dependencies also resolve to one provider today.
  `All` and by-id selection across that boundary are future work.
- **Health check, reconnect, and restart.**
  A launched extension is connected once.
  If the process dies, callers get gRPC errors until the daemon restarts.
  There is no watchdog, reconnect loop, or restart policy yet.
- **Public importable packages.**
  The framework lives under `internal/`, so another module cannot import a point or its generated client.
  Publishing a package such as `github.com/moby/extensions` would allow out-of-tree points and Go clients for exposed points.
  That would also freeze the Go API as a compatibility promise, so the primitives need to settle first.
