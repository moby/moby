# Authoring extension points and extensions

This guide explains how to add extension points and write extensions in Moby.
It covers two tasks:

- **[Adding a new extension point](#adding-a-new-extension-point)** — define a Go contract, generate the wire code, call the point from the engine, and allow separate-binary providers.
- **[Writing an extension](#writing-an-extension)** — implement a point as a built-in extension or as a separate binary.

For the model, read [GLOSSARY.md](./GLOSSARY.md), [PRINCIPLES.md](./PRINCIPLES.md), and [DESIGN.md](./DESIGN.md).
For examples, read [EXAMPLES.md](./EXAMPLES.md).

Use the real points in `extpoints/` as references.
`createspec/v0` is a real engine hook.
`containercreate/v0` is another engine point.
`internal/extensions/servicegrpc/v0` is the standard socket-exposure point used by the SDK and host.
`internal/extensions/example/greeter/v0` is a small service point used by tests.

## The shape of a point on disk

A point lives in `extpoints/<area>/<name>/v<N>/`.
It has one hand-written Go contract, one hand-written generate file, and generated wire files.

```
extpoints/createspec/v0/
  createspec.go              # you write: interface, messages, Point, helpers
  gen.go                     # you write: package doc and //go:generate
  protogen/                  # generated, do not edit
    create_spec_hook.proto   # generated from the Go contract
    *.pb.go                  # generated proto messages
    *_grpc.pb.go             # generated gRPC stubs
    wire.gen.go              # generated ClientPoint, ServerPoint, adapters
```

The Go file is the source of truth.
Do not hand-edit the `.proto` file or anything in `protogen/`.
"Go-first" means a maintainer writes an ordinary Go interface, and the wire format is generated from it.

## Adding a new extension point

### 1. Write the Go contract

Create `extpoints/<area>/<name>/v0/<name>.go`.
A new point starts at `v0`.
A `.v0` point is experimental and may change without backward compatibility until it is promoted to `v1`.

The contract contains:

- the provider interface;
- the message structs passed by the interface;
- the `Point` value;
- helper functions the engine calls.

A small service point looks like this:

```go
// Package greeterv0 ...
package greeterv0

import (
	"context"

	"github.com/moby/moby/v2/internal/extensions"
)

// Greeter is the provider interface.
type Greeter interface {
	Greet(ctx context.Context, req *HelloRequest) (*HelloReply, error)
}

// HelloRequest is the request message.
type HelloRequest struct {
	Name string `pb:"1"`
}

// HelloReply is the response message.
type HelloReply struct {
	Message string `pb:"1"`
}

// Point binds the interface to a namespaced, versioned id.
var Point = extensions.DefinePoint[Greeter]("org.mobyproject.extension.example.greeter.v0")

// Greet resolves the provider and calls it.
func Greet(ctx context.Context, resolver extensions.Resolver, req *HelloRequest) (*HelloReply, error) {
	g, err := Point.Single(resolver)
	if err != nil {
		return nil, err
	}
	return g.Greet(ctx, req)
}
```

Make these decisions for the point:

- **Id and version.**
  Engine points use ids such as `org.mobyproject.extension.<area>.<name>.v0`.
  Vendor points use their own reverse-DNS namespace.
- **Resolution shape.**
  Provide helpers that call `Point.Single`, `Point.All`, or `Point.ByExtension`.
  The engine should call your helper, not use `Point` directly.
  This keeps call and veto behavior in one place.
- **Ordering.**
  Fan-out order is undefined.
  If order matters, the point must define it.
  Prefer phases, such as `Update` followed by `Validate`, over relying on provider order.
- **Call and veto behavior.**
  Decide whether calls block, whether they are fire-and-forget, and whether a provider can reject the operation.
- **Method shape.**
  Point methods should be unary request/response calls: `M(ctx, *Req) (*Resp, error)`.
- **Dependencies.**
  Decide what points a provider may call back into and what other extensions it may need.
  Keep the graph acyclic.
  Mark optional or lazy dependencies deliberately.
- **Failure handling.**
  Decide fail-open or fail-closed behavior.
  Security and veto points usually need fail-closed behavior.
  For fail-closed points, give each provider call its own deadline.
  Create the timeout around the call and cancel it before moving to the next provider.
  Out-of-process providers receive the deadline through gRPC, which enforces it.
  In-process providers receive the same context but are only trusted to honor it — the deadline is not forced on them.
  Do not run an in-process provider in a goroutine just to abandon it on timeout.
  That can leave a goroutine mutating shared engine state after the caller continues.
  So the timeout only bounds out-of-process providers; state a security veto's guarantee accordingly, and do not claim it protects against an in-process provider that hangs.
- **Messages.**
  Use plain structs with `pb:"N"` tags.
  Field numbers are the wire contract.
  Keep numbers stable within a version and never renumber a `.v1` field.
  Prefer passing standard data formats directly when they already exist.
  For example, `createspec` carries the OCI runtime spec as raw JSON bytes instead of remodelling it in proto.

Supported message fields are intentionally limited:

- scalar values, `string`, and `[]byte`;
- repeated scalars such as `[]string`;
- `map[string]string` and similar string-keyed maps of scalar values;
- nested messages as pointers, such as `*Other`;
- repeated nested messages, such as `[]Other`.

Use a pointer for a single nested message.
A value field for a nested message is rejected.
The generator errors on unsupported shapes instead of generating ambiguous wire code.

For points that the engine offers to extensions, such as socket exposure or info, the interface shape is the same.
Only the caller direction changes.
See the standard points in [EXAMPLES.md](./EXAMPLES.md#standard-points).

### 2. Add the generate directive

Create `gen.go` next to the contract.
It contains the package doc and the `//go:generate` line.
Copy an existing file and change the `-dir`, `-import`, and `-proto` paths.

```go
//go:generate bash -c "cd ../../.. && go run ./internal/extensions/cmd/mobyextgen -dir extpoints/<area>/<name>/v0 -import github.com/moby/moby/v2/extpoints/<area>/<name>/v0 -proto <name>.proto && protoc --go_out=. --go_opt=module=github.com/moby/moby/v2 --go-grpc_out=. --go-grpc_opt=module=github.com/moby/moby/v2 -I . extpoints/<area>/<name>/v0/<name>.proto"

// Package <name>v0 is the <name> extension point contract, written Go-first ...
package <name>v0
```

`mobyextgen` reads the Go contract and writes the `.proto` file plus `wire.gen.go`.
`protoc` then writes the proto messages and gRPC stubs.
The contract package itself does not import protobuf packages.
Generated code goes in `protogen/`.

### 3. Generate

Run generation through the pinned toolchain:

```console
$ make generate-extensions
```

This runs in Docker with the `protoc` and plugin versions used by the repo.
It copies generated files back into the tree.
CI runs `make validate-generate-extensions` and fails if committed generated files do not match a fresh run.
Always commit generated output.

You can iterate on one package with `go generate ./extpoints/<area>/<name>/v0/`, but that requires `protoc` and plugins on your `PATH`.
The make target is the reproducible path.

### 4. Call the point from an engine flow

A point does nothing until the engine calls it.
Import the contract package and call its helper.
Pass the host as the `Resolver`.
The daemon's `*host.Host` satisfies `extensions.Resolver`.

The create-spec hook is called from `daemon/createspec.go` after the OCI spec is built:

```go
func (daemon *Daemon) runCreateSpecHooks(ctx context.Context, c *container.Container, spec *specs.Spec) error {
	if daemon.extensionHost == nil {
		return nil
	}
	// ... marshal spec into req ...
	adjusted, err := createspecv0.CreateSpec(ctx, daemon.extensionHost, req)
	if err != nil {
		return err
	}
	// ... apply adjusted spec ...
	return createspecv0.Validate(ctx, daemon.extensionHost, req)
}
```

If no extension implements a fan-out point, the helper resolves zero providers and the engine flow is a no-op.
Calling such a point is safe.

### 5. Let separate-binary extensions implement it

Steps 1 through 4 are enough for in-process providers.
To allow out-of-process providers, add the generated `ClientPoint` to `clientProviders()` in `daemon/extensions.go`.

```go
func clientProviders() []clientpoint.Registration {
	return []clientpoint.Registration{
		containercreatepb.ClientPoint,
		createspecpb.ClientPoint,
		<name>pb.ClientPoint, // add this
	}
}
```

No other host-side change is needed.
The host uses `ClientPoint` to build an in-process caller from the gRPC connection to the launched extension.

This list is the boundary for what a launched extension may provide.
If an extension declares a point that is not listed, the daemon rejects it because it has no client wiring for that point.
Any installed extension may provide any listed point.
The operator controls that by controlling what is installed.
See the [discovery security model](./DESIGN.md#security-model-for-discovery).

A gRPC service published for external clients is not a point in this list.
That case uses socket exposure.

#### Socket-exposed services are not points

An extension can publish its own gRPC API on `docker.sock`.
For example, Docker Compose could expose an API for external clients.
That service is not an extension point called by the daemon.
The daemon only forwards bytes by service name and does not import the proto.

The extension opts in by implementing `service.grpc` and registering its gRPC services:

```go
type expose struct{}

func (expose) RegisterServices(r grpc.ServiceRegistrar) {
	mypb.RegisterMyServiceServer(r, impl) // or mypb.ServerPoint.Register(r, impl)
}

var Extension = extensions.New(extensions.Declaration{
	ID:        "com.example.myext",
	Providers: []extensions.Provider{servicegrpcv0.Point.Provide(expose{})},
})
```

The framework supplies the registrar.
The same code works in-process and out-of-process:

- **In-process:** the daemon registers the service on its own gRPC server, so it is served on the socket directly.
- **Out-of-process:** the SDK registers the service on the extension server and reports service names to the daemon.
  The daemon proxies matching calls to the extension.

A binary registers the extension and the `ServerPoint` for every point the extension provides:

```go
srv := sdk.NewServer()
srv.Register(ext,
	servicegrpcv0.ServerPoint,
	mypointpb.ServerPoint, // include every other point this extension provides
)
srv.Listen(ctx)
```

Service names are captured from registration, so do not list them by hand.
Point id, proto package, gRPC service name, and CLI route are different identifiers.
See [DESIGN.md](./DESIGN.md#identifiers).

## Wire compatibility and current limits

Messages use proto3 on the wire.
That affects what a contract can express and how it can change.
A `.v1` point promises compatibility within its version.

### Presence

Proto3 scalar fields have no presence.
A zero value such as `""`, `0`, or `false` looks the same as an unset value on the wire.
A scalar cannot represent both "leave unchanged" and "set to the zero value".

When that distinction matters, do not use a bare scalar.
Use one of these shapes instead:

- wrap the value in a message field such as `*Thing`, because message fields have presence;
- model the state explicitly, such as a separate boolean flag or an interpreted `bytes` payload.

### Not yet supported

The generator supports a narrow set of types on purpose.
It does not support optional scalars, `oneof`, enums, or well-known types such as timestamp and duration.
Model around those limits for now.
For example, use a Unix `int64` time, a documented `string` for enum-like values, or a `bytes` / JSON payload for structured data.

There is also no typed error schema.
A provider returns a Go `error`, which is carried over gRPC as a status.
The point contract explains what that error means, such as a veto.

### Evolving a point within its version

Field numbers are the compatibility contract.
The `pb:"N"` tags must be handled carefully.

- **Add** a field with a new, unused number.
  Old peers ignore it.
  New peers see the zero value when talking to old peers.
  Remember the presence rule.
- **Never renumber** a field.
  Never reuse a number.
  Never change a field's type.
  Any of these silently breaks the wire format.
- **Deleting** a field frees its name but not its number.
  Treat removed numbers as burned.
  The generator does not yet track reserved numbers.
- **Breaking changes** require a new point version.
  That means a new `.vN`, not an edit in place.
  `.v0` is experimental and may change freely.

## Writing an extension

An extension is an ordinary `extensions.Extension` value.
There is no `func init()` and no global registry.
Something constructs the extension and passes it to a host or SDK server.
Importing the package does nothing by itself.

### Stateless: wrap a Declaration

If the extension has no state, wrap a `Declaration` with `extensions.New`:

```go
var Extension = extensions.New(extensions.Declaration{
	ID:        "org.example.no-privileged.v1",
	Providers: []extensions.Provider{createspecv0.Point.Provide(&policy{})},
})
```

`Point.Provide(impl)` ties the implementation to the point.
The implementation only has to satisfy the point's Go interface.

### Stateful: implement the interface

If the extension owns state, implement `extensions.Extension` on your type.
This lets the same object configure itself, provide points, and shut down.

```go
var Extension extensions.Extension = &Bridge{}

func (b *Bridge) Declaration() extensions.Declaration {
	return extensions.Declaration{
		ID:        ExtensionID,
		Providers: []extensions.Provider{createspecv0.Point.Provide(b)},
		Init:      b.init,
		Shutdown:  b.Stop,
	}
}
```

`Init` receives the extension config and a resolver.
The config is keyed by extension id in `daemon.json`.
The resolver gives access to declared dependencies.
Declare dependencies and conflicts in the `Declaration`.
The broker initializes dependencies before dependents.

For out-of-process extensions, the resolver calls dependencies through the daemon over a callback channel.
The binary must declare the client wiring for dependency points:

```go
srv := sdk.NewServer()
srv.Register(ext)
srv.Depends(volumedriverpb.ClientPoint) // one per dependency point it will call
srv.Listen(ctx)
```

The host must also offer those points as dependencies.
The daemon lists a point's `ServerPoint` in `dependencyProviders()` in `daemon/extensions.go`.
A dependency on an extension-defined point works only if the daemon supports the generated wiring.

The operator configures extensions under `extension-config` in `daemon.json`:

```json
{
  "extension-config": {
    "com.example.myext": { "some_key": "value" }
  }
}
```

The same config reaches the extension in-process during `Init` or out-of-process through the startup handshake.

### Run it in-process (built-in)

Register a built-in by adding it to `builtinExtensions()` in `daemon/extensions.go`.
Select it from daemon config as needed.

```go
func builtinExtensions(cfg *config.Config) []extensions.Extension {
	var exts []extensions.Extension
	if cfg.SomeFeatureEnabled {
		exts = append(exts, somepkg.Extension)
	}
	return exts
}
```

A built-in uses the same registration path as a launched binary.
Its config reaches it by id through `host.Options.ExtensionConfig`.

### Run it out-of-process (separate binary)

Write a `main` that builds an SDK server, registers the same extension value, and listens.
The binary also passes the generated `ServerPoint` for each point it provides.
The in-process host gets the implementation directly, but the binary needs the gRPC server side.

```go
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := sdk.NewServer()
	if err := srv.Register(myext.Extension, createspecpb.ServerPoint); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := srv.Listen(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

Pass one `ServerPoint` for each point the extension provides.
The SDK runs the same `Init` and `Shutdown` lifecycle as the in-process host.
Only packaging differs.

**`stdout` is reserved for the runtime handshake.**
Log to `stderr`.
At launch, the daemon writes startup config to the binary's stdin and waits for one readiness line on stdout.
The SDK writes that line once it is listening.
Any other stdout output before then corrupts the handshake and launch fails.
The daemon captures stderr and includes it in its own logs.

Deploy the binary with the extension id as its name.
Put it in the extensions directory, which defaults to `/usr/libexec/docker/moby-extensions/`.
`--extension-dir` overrides the directory.
Rootless mode uses the user's libexec home.
On Windows, the binary can use `<id>.exe`.

The daemon discovers and launches extension binaries at startup.
The point must be listed in [`clientProviders()`](#5-let-separate-binary-extensions-implement-it), or the daemon rejects it.
There is no watchdog yet.
If an extension process dies, callers get gRPC errors until the daemon restarts.
Health checks, reconnect, and restart are future work in [ROADMAP.md](./ROADMAP.md).

## Quick reference

| Task | Where | What |
|---|---|---|
| Define a point | `extpoints/<area>/<name>/v0/<name>.go` | Go interface, `pb`-tagged messages, `DefinePoint`, and helpers |
| Wire the point | `extpoints/<area>/<name>/v0/gen.go` | package doc and `//go:generate` |
| Generate | `make generate-extensions` | regenerate `protogen/`; CI validates the result |
| Invoke the point | the relevant engine flow | call the contract helper with the host as `Resolver` |
| Support out-of-process | `daemon/extensions.go` → `clientProviders()` | add `<name>pb.ClientPoint` |
| Write an extension | anywhere | use `extensions.New(Declaration{…})` or implement `Extension` |
| Run it built-in | `daemon/extensions.go` → `builtinExtensions()` | append the extension value |
| Run it as a binary | a `cmd/<name>/main.go` | `sdk.NewServer()`, `Register(ext, <name>pb.ServerPoint)`, then `Listen` |
