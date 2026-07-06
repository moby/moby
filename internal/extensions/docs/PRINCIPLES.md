# Extensions — Principles

- **Everything is an extension point.**
  A point is a versioned, namespaced interface id, such as `org.mobyproject.extension.container.create_hook.v1`.
  There is no separate hook concept.
  A hook is just a point the engine calls during one of its flows.
  An extension is anything that implements one or more points.
- **Points are uniform.**
  A point works the same way no matter who defines it or who calls it.
  The same interface, provider model, and routing path are used for engine calls and extension-to-extension calls.
- **Socket exposure is opt-in, and it is also a point.**
  By default, an extension is reachable only inside the daemon.
  An extension can publish its own gRPC services on `docker.sock` by implementing `org.mobyproject.extension.service.grpc.v0`.
  The daemon forwards those services by name.
  It does not need to know their proto files.
  This lets an extension add API surface without making it a built-in daemon API.
  These services are served on the raw gRPC endpoint alongside the daemon's own gRPC services (such as BuildKit), so, like those, they are not gated by authorization plugins — those apply to the REST API.
  An extension that needs access control must enforce it itself.
- **Extensions do not depend on their location.**
  Extension code should not care whether it is compiled into the daemon or runs as a separate process.
  The runtime owns the transport choice.
- **Dependencies are typed.**
  An extension declares what it needs by point or by extension id.
  The broker resolves those needs before initialization.
  A point dependency lets the extension fetch providers later.
  An extension dependency gives it one named extension, initialized first.
  Both in-process and out-of-process extensions receive a resolver they can use to call dependencies.
- **Broker plus dependency injection.**
  Like containerd, extensions register, declare dependencies, and initialize in dependency order.
  Unlike containerd, those dependencies can point to out-of-process extensions without changing the caller.
- **Registration is explicit.**
  There is no package-level `func init()` registration.
  A host chooses what to run by passing extension values to the runtime.
  Importing an extension package does nothing by itself.
  This keeps the active set clear, testable, and free of import-order side effects.
- **Extensions replace legacy plugins.**
  Network, volume, and log drivers become extension points.
  The old plugin system goes away.
