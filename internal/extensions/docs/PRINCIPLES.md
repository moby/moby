# Extensions — Principles

- **Everything is an extension point.**
  A point is a versioned, namespaced interface id, such as `org.mobyproject.extension.container.create_hook.v1`.
  There is no separate hook concept.
  A hook is just a point the engine calls during one of its flows.
  An extension is anything that implements one or more points.
- **Points are uniform.**
  A point works the same way no matter who defines it or who calls it.
  The same interface, provider model, and routing path are used for engine calls and extension-to-extension calls.
- **Dependencies are typed.**
  An extension declares what it needs by point or by extension id.
  The broker resolves those needs before initialization.
  A point dependency lets the extension fetch providers later.
  An extension dependency gives it one named extension, initialized first.
  Extensions receive a resolver they can use to call their dependencies.
- **Broker plus dependency injection.**
  Like containerd, extensions register, declare dependencies, and initialize in dependency order.
- **Registration is explicit.**
  There is no package-level `func init()` registration.
  A host chooses what to run by passing extension values to the runtime.
  Importing an extension package does nothing by itself.
  This keeps the active set clear, testable, and free of import-order side effects.
- **Extensions replace legacy plugins.**
  Network, volume, and log drivers become extension points.
  The old plugin system goes away.
