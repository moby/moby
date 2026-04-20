# AGENTS.md

## Project overview

smithy-go is the Go code generator and runtime for [Smithy](https://smithy.io/).
It has two major components:

1. **Codegen** (`codegen/`) — A Smithy build plugin written in Java that
   generates Go client/server/shape code from Smithy models.
2. **Runtime** (`./`, top-level Go module) — The Go packages that generated
   code depends on at runtime.

The primary downstream consumer is
[aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2).

## Repository layout

```
.                               # Root Go module (github.com/aws/smithy-go)
├── auth/                       # Auth identity + scheme interfaces
│   └── bearer/                 # Bearer token auth
├── aws-http-auth/              # Separate module: AWS SigV4/SigV4A HTTP signing
├── codegen/                    # Java/Gradle: Smithy code generator
│   ├── smithy-go-codegen/      # Main codegen source (Java)
│   └── smithy-go-codegen-test/ # Codegen integration tests
├── container/                  # Generic container types
├── context/                    # Context helpers
├── document/                   # Smithy document type abstraction
│   └── json/                   # JSON document codec
├── encoding/                   # Wire format encoders/decoders
│   ├── cbor/                   # CBOR (used by rpcv2Cbor)
│   ├── httpbinding/            # HTTP binding serde helpers
│   ├── json/                   # JSON encoder/decoder
│   └── xml/                    # XML encoder/decoder
├── endpoints/                  # Endpoint resolution types
├── internal/                   # Internal utilities (singleflight, etc.)
├── io/                         # I/O helpers
├── logging/                    # Logging interfaces
├── metrics/                    # Metrics interfaces
│   └── smithyotelmetrics/      # Separate module: OpenTelemetry metrics adapter
├── middleware/                 # Middleware stack (the core of the operation pipeline)
├── ptr/                        # Pointer-to/from-value helpers
├── testing/                    # Test assertion helpers for generated protocol tests
│   └── xml/                    # XML comparison utilities
├── time/                       # Smithy timestamp format helpers
├── tracing/                    # Tracing interfaces
│   └── smithyoteltracing/      # Separate module: OpenTelemetry tracing adapter
└── transport/
    └── http/                   # HTTP request/response types and middleware
```

## Building and testing

### Runtime (Go)

```bash
# Run unit tests
make unit
```

### Codegen (Java)

```bash
# Build and test codegen
cd codegen && ./gradlew build

# Publish to local Maven for downstream use
cd codegen && ./gradlew publishToMavenLocal
```

The codegen artifact version is fixed at `0.1.0` and is not published to
Maven Central — you **MUST** `publishToMavenLocal`.

## Runtime architecture

### Middleware stack

The operation pipeline is built on a middleware stack defined in `middleware/`.
Steps execute in order: Initialize → Serialize → Build → Finalize →
Deserialize. Each step is a `middleware.Step` that holds an ordered list of
middleware. The codegen generates middleware registrations for each operation.

### Encoding packages

Each wire format has its own encoder/decoder under `encoding/`. These are
low-level — they produce/consume raw tokens or values, not full Smithy shapes.
Generated serde code calls into these packages.

## Codegen: GoWriter and template system

GoWriter extends Smithy's `SymbolWriter` and is the primary mechanism for
generating Go source. It has **two distinct writing styles** that must not be
confused.

### Style 1: Positional args (`writer.write` / `writer.openBlock`)

Inherited from `SymbolWriter`. Arguments are positional and referenced with
`$`-prefixed format characters. Each `$X` consumes the next argument in order.

Format characters:
- `$L` — Literal (toString). Strings, names, anything that should be inserted
  verbatim.
- `$S` — String, quoted. Wraps the value in Go double-quotes.
- `$T` — Type (Symbol). Inserts the symbol name and auto-adds its import.
- `$P` — Pointable type (Symbol). Like `$T` but prepends `*` if the symbol is
  marked pointable.
- `$W` — Writable. Evaluates a `Writable` (lambda/closure) inline.
- `$D` — Dependency. Adds a `GoDependency` import, expands to empty string.

Numbered variants (`$1L`, `$2T`, etc.) allow reusing the same argument
multiple times. The number is 1-indexed and refers to the position in the
argument list:

```java
// $1L is used twice, $2L once — only 2 args needed
writer.write("type $1L struct{}\nvar _ $2L = (*$1L)(nil)",
    DEFAULT_NAME, INTERFACE_NAME);
```

`openBlock`/`closeBlock` manage indentation for braced blocks. Arguments are
positional:

```java
writer.openBlock("func (c $P) $T(ctx $T) ($P, error) {", "}",
    serviceSymbol, operationSymbol, contextSymbol, outputSymbol,
    () -> {
        writer.write("return nil, nil");
    });
```

### Style 2: Named template args (`goTemplate` / `writeGoTemplate`)

Uses `$name:X` syntax where `name` is a key in a `Map<String, Object>` and `X`
is the format character. Arguments are passed as one or more maps. This is the
**preferred style for new code** — it is more readable and less error-prone
than positional args.

```java
return goTemplate("""
    func $name:L(v $cborValue:T) ($type:T, error) {
        return $coercer:T(v)
    }
    """,
    Map.of(
        "name", getDeserializerName(shape),
        "cborValue", SmithyGoTypes.Encoding.Cbor.Value,
        "type", symbolProvider.toSymbol(shape),
        "coercer", coercer
    ));
```

Rules:
- `goTemplate(String, Map...)` is a **static** method that returns a
  `Writable` (a `Consumer<GoWriter>` lambda). It does NOT write immediately.
- `writeGoTemplate(String, Map...)` is an **instance** method that writes
  immediately to the writer.
- Maps are merged into the writer's context scope for the duration of the
  template. Multiple maps can be passed and are applied in order.
- The writer pre-populates common symbols in context: `fmt.Sprintf`,
  `fmt.Errorf`, `errors.As`, `context.Context`, `time.Now`.

### Composing writables

- `ChainWritable` — Collects multiple `Writable`s and composes them with
  newlines between each. Use `.compose()` (with newlines) or
  `.compose(false)` (without).

### Symbol constants

For symbols, use `SmithyGoDependency.*.valueSymbol("Name")` or
`SmithyGoDependency.*.pointableSymbol("Name")`.

