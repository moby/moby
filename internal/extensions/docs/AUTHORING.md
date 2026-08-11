# Authoring extension points and extensions

This is the procedural guide.
Read [DESIGN.md](./DESIGN.md) for current rules and the [glossary](./README.md#glossary) for terms.
No engine hook points are available as implementation references yet, so the snippets below are illustrative.

## Point authoring

### Files and source of truth

Create `extpoints/<area>/<name>/v0/`.
A point has one hand-written Go contract and generated wire files:

```
extpoints/<area>/<name>/v0/
  <name>.go                 # package doc, //go:generate, contract, helpers
  <service>.proto            # generated from the Go contract
  protogen/                  # generated; do not edit
    <service>.pb.go          # generated proto messages
    wire.gen.go              # service, client, ClientPoint, ServerPoint,
                             # adapters, and conversions
```

The Go file is the source of truth.
Do not hand-edit the `.proto` file or anything in `protogen/`; `mobyextgen` regenerates them.

### 1. Write the Go contract

Create `extpoints/<area>/<name>/v0/<name>.go`.
New points start at `v0`, which is experimental and may change without compatibility until promoted to `v1`.
The contract contains the provider interface, message structs, `Point` value, and helpers the engine calls.

This small service point shows the required shape:

```go
// Package greeterv0 is the greeter extension point contract.
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

// Point binds the interface to a namespaced, versioned id. The pragma names the
// point's gRPC service; it is part of the wire contract, so it lives here rather
// than in a generator flag.
//
//mobyextgen:service=Greeter
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

Before coding, record the id and version, resolution shape and cardinality, ordering, call/veto semantics, dependencies, and failure policy.
The detailed rules are [DESIGN.md](./DESIGN.md#resolution-ordering-and-lifecycle); the authoring-specific choices are:

- Use `org.mobyproject.extension.<area>.<name>.v0` for an engine point or a vendor reverse-DNS namespace, and a new `.vN` for a breaking change.
- Call `Point.Single`, `Point.All`, or `Point.ByExtension` from a helper rather than from engine code.
  Use `DefineSinglePoint` for a deciding point; the generated `ClientPoint` makes the host reject two installed providers.
  Built-ins yield to installed providers.
  Runtime fallback and failure behavior belongs to the point's call helper and must be stated in its contract.
  Omit a built-in point's `ClientPoint` from `clientProviders()` to close it to replacement; a launched declaration is then rejected at client wiring.
- Prefer unary `M(ctx, *Req) (*Resp, error)` methods and explicit phases such as `Update` followed by `Validate`; keep dependencies acyclic and optional when absence is valid.
- Choose fail-open or fail-closed behavior.
  Security and veto points normally fail closed.
  Use `extensions.Policy` with `Each` or `Fold`:

  ```go
  var policy = extensions.Policy{Timeout: 30 * time.Second}

  func Validate(ctx context.Context, resolver extensions.Resolver, req *Request) error {
      vetoing := policy
      vetoing.Action = "vetoed the start"
      return extensions.Each(ctx, Point, resolver, vetoing,
          func(ctx context.Context, p Provider) error { return p.Validate(ctx, req) })
  }
  ```

  `Policy.Timeout` gives each provider a fresh deadline and `FailOpen` skips a failure.
  Errors identify their extension.
  Each call's gRPC deadline is enforced out of process; an in-process provider only receives a context and must honor it, because abandoning a direct call could leave it mutating shared state.
  Do not claim this timeout protects against a hanging in-process provider.
- Use plain `pb:"N"`-tagged structs, stable field numbers, and existing data formats directly.

Supported fields are scalar values, `string`, `[]byte`, repeated scalars such as `[]string`, string-keyed scalar maps, pointer nested messages such as `*Other`, and repeated nested messages such as `[]Other`.
A single nested message must be a pointer; the generator rejects a value field.

When deleting a field, burn its number and record it on the message:

```go
//mobyextgen:reserved=2 was 'exclusive'; exposure covers sole-ownership
type PointDeclaration struct {
	ID string `pb:"1"`
}
```

The generator emits `reserved 2;` and rejects later reuse.
For proto3 presence, unsupported types, and the complete compatibility rules, see [DESIGN.md#wire-contract-and-compatibility](./DESIGN.md#wire-contract-and-compatibility).

Points the engine offers to extensions use the same interface shape; only caller direction changes.

### 2. Add generation

Put the directive at the top of the contract file.
It is identical for every point; there are no paths to adjust:

```go
//go:generate go run github.com/moby/moby/v2/internal/extensions/cmd/mobyextgen

// Package <name>v0 is the <name> extension point contract, written Go-first ...
package <name>v0
```

`mobyextgen` reads the Go contract and writes the `.proto`, protobuf message code, and `wire.gen.go`; it derives the import path from `go.mod` and the proto file name from `mobyextgen:service`.
The contract package does not import protobuf packages; generated code belongs in `protogen/`.

### 3. Generate and validate

Run the pinned toolchain:

```console
$ make generate-extensions
```

This regenerates every contract and copies the result back into the tree.
CI runs `make validate-generate-extensions` and fails if committed output does not match a fresh run.
Commit generated output.

Generation needs only the Go toolchain: no `protoc`, plugins, or other `PATH` tools.
To regenerate one point use:

```console
$ go generate ./extpoints/<area>/<name>/v0/
```

To reproduce the CI scope use:

```console
$ go generate ./extpoints/... ./internal/extensions/...
```

The make target pins the Go version to make validation hermetic, not because generation needs a container.

### 4. Call the point from an engine flow

Import the contract and call its helper with the host as `extensions.Resolver`.
The daemon's `*host.Host` satisfies that interface.
A point with no providers resolves zero providers and is a safe no-op.
Call the helper at the engine boundary where its input is complete.
A security policy point must inspect the authoritative data used by the protected operation so an earlier or partial representation cannot bypass it.

### 5. Support separate-binary providers

Steps 1–4 are sufficient for in-process providers.
To support a launched provider, add its generated `ClientPoint` to `clientProviders()` in `daemon/extensions.go`:

```go
func clientProviders() []clientpoint.Registration {
	return []clientpoint.Registration{
		<name>pb.ClientPoint, // add this
	}
}
```

`ClientPoint` builds an in-process caller from the gRPC connection.
This list is the boundary for launched providers: an unlisted declared point is rejected, while any installed extension may provide a listed point.
See [DESIGN.md#discovery-security](./DESIGN.md#discovery-security).

An externally published gRPC service is not added to this list.
It uses socket exposure instead.

### Socket-exposed services

Socket exposure is an extension's own gRPC API, not a daemon-called point.
The daemon forwards raw calls by service name without importing the proto.
Opt in with `service.grpc` and register through the supplied registrar:

```go
type expose struct{}

func (expose) RegisterServices(r grpc.ServiceRegistrar) {
	mypb.RegisterMyServiceServer(r, impl) // or mypb.ServerPoint.Register(r, impl)
}

var Extension = extensions.New(extensions.Declaration{
	ID:        "com.example.myext.v1",
	Providers: []extensions.Provider{servicegrpcv0.Point.Provide(expose{})},
})
```

The same implementation works in both modes: in-process, the daemon supplies its gRPC server; out-of-process, the SDK supplies the extension server, records names, and the daemon proxies matching calls.

An out-of-process binary registers `ServerPoint` for every point it provides:

```go
srv := sdk.NewServer()
srv.Register(ext,
	servicegrpcv0.ServerPoint,
	mypointpb.ServerPoint, // include every other point this extension provides
)
srv.Listen(ctx)
```

Service names are captured from registration, so do not list them manually.
Point id, proto package, gRPC service name, and CLI route remain different identifiers; see [DESIGN.md#identifiers](./DESIGN.md#identifiers).

## Writing an extension

An extension is an `extensions.Extension` value; constructing and passing it to a host or SDK server activates it.
There is no global registry or `func init()`.

### Stateless extension

Wrap a declaration with `extensions.New`:

```go
var Extension = extensions.New(extensions.Declaration{
	ID:        "org.example.myext.v1",
	Providers: []extensions.Provider{mypointv0.Point.Provide(&provider{})},
})
```

`Point.Provide(impl)` ties an implementation to a point; the implementation only needs to satisfy that point's Go interface.

### Stateful extension

Implement `extensions.Extension` when the extension owns state.
The same object can configure itself, provide points, and shut down:

```go
var Extension extensions.Extension = &Bridge{}

func (b *Bridge) Declaration() extensions.Declaration {
	return extensions.Declaration{
		ID:        ExtensionID,
		Providers: []extensions.Provider{mypointv0.Point.Provide(b)},
		Init:      b.init,
		Shutdown:  b.Stop,
	}
}
```

`Init` receives the extension config and a resolver.
Config is keyed by id in `daemon.json`; declare dependencies and conflicts in the declaration.
The broker initializes dependencies before dependents.

For an out-of-process extension, wire every dependency point it will call:

```go
srv := sdk.NewServer()
srv.Register(ext, mypointpb.ServerPoint) // include each provided point
srv.Depends(volumedriverpb.ClientPoint) // one per dependency point it will call
srv.Listen(ctx)
```

The host must offer the same points as dependencies by listing their generated `ServerPoint` in `dependencyProviders()` in `daemon/extensions.go`.
A dependency on an extension-defined point works only when the daemon supports that wiring.
Cross-process dependency lookup currently exposes one provider; `All` and by-id selection are deferred.

Configure extensions under `extension-config` in `daemon.json`:

```json
{
  "extension-config": {
    "com.example.myext.v1": { "some_key": "value" }
  }
}
```

The same entry reaches an in-process `Init` or a separate binary through the startup handshake.

### Run in-process

Register a built-in by adding it to `builtinExtensions()` in `daemon/extensions.go`; select it from daemon config as needed:

```go
func builtinExtensions(cfg *config.Config) []extensions.Extension {
	var exts []extensions.Extension
	if cfg.SomeFeatureEnabled {
		exts = append(exts, somepkg.Extension)
	}
	return exts
}
```

Built-ins use the same registration path as launched binaries.
Their config is delivered by id through `host.Options.ExtensionConfig`.

### Run out-of-process

Write a `main` that builds an SDK server, registers the extension, and passes a generated `ServerPoint` for each point it provides:

```go
func main() {
	sdk.Main(myext.Extension, mypointpb.ServerPoint)
}
```

Use `sdk.NewServer` instead when the binary needs `Depends` or other server setup.
`sdk.Main` handles daemon stop signals, serves the extension, and exits non-zero on failure.
The SDK runs the same `Init` and `Shutdown` lifecycle as an in-process host; only packaging differs.

`stdout` is reserved for the runtime handshake.
Log to `stderr`.
The daemon writes startup config to stdin and waits for one `ready\n` line on stdout; the SDK writes it once listening.
Any other pre-readiness stdout corrupts the handshake and fails launch.
The daemon captures stderr in its logs.

Deploy the binary with the extension id as its name, in the extensions directory, which defaults to `/usr/libexec/docker/moby-extensions/`.
`--extension-dir` overrides it; rootless mode uses the user's libexec home; on Windows use `<id>.exe`.
The daemon discovers and launches binaries at startup.

There is no watchdog yet.
If the process dies, callers get gRPC errors until the daemon restarts.
Health checks, reconnect, and restart are future work in [ROADMAP.md](./ROADMAP.md).

## Author checklist

- [ ] Put the hand-written contract under `extpoints/<area>/<name>/v<N>/`.
- [ ] Use a valid point id, stable `pb` field numbers, and a service pragma.
- [ ] Define resolution/cardinality, ordering, call/veto, dependency, and failure behavior in the point contract.
- [ ] Generate and commit `.proto` and `protogen/` output.
- [ ] Call a helper from the engine flow, passing the host resolver.
- [ ] Add generated `ClientPoint` wiring for separate-binary providers.
- [ ] Add dependency `ServerPoint` wiring when separate binaries call engine points.
- [ ] Register every provider's `ServerPoint` in a separate binary.
- [ ] Keep handshake output on stdout and logs on stderr.
- [ ] Install one correctly named, non-world-writable binary in the trusted extension directory.

## Quick reference

| Task | Where | What |
|---|---|---|
| Define a point | `extpoints/<area>/<name>/v0/<name>.go` | Go interface, `pb` messages, `DefinePoint`, helpers |
| Wire it | `extpoints/<area>/<name>/v0/<name>.go` | package doc and identical `//go:generate` |
| Generate | `go generate ./extpoints/<area>/<name>/v0/` | regenerate `.proto` and `protogen/` |
| Invoke it | relevant engine flow | call the contract helper with the host resolver |
| Support a binary | `daemon/extensions.go` → `clientProviders()` | add `<name>pb.ClientPoint` |
| Write an extension | anywhere | `extensions.New(Declaration{…})` or `Extension` |
| Run it built-in | `daemon/extensions.go` → `builtinExtensions()` | append the extension value |
| Run it as a binary | `cmd/<name>/main.go` | `sdk.Main(ext, <name>pb.ServerPoint)` |
