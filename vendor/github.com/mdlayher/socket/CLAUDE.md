# CLAUDE.md

Guidance for AI agents working in this repository.

## Verification

Run these on every Go change, alongside `go build ./...` and
`go test -race ./...`:

- `gofumpt -l .` (keep stderr visible so a bad path can't look like a clean
  run)
- `go vet ./...`
- `GOOS=darwin go vet ./...` (CI tests on macOS as well as Linux; any file
  without a `//go:build linux` constraint must build there)
- `staticcheck ./...` (if the system binary is too old for the toolchain,
  use `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`)
- `gopls check -severity=hint <changed files>` (catches unusedfunc,
  modernize, typeargs, and other hint-level findings vet and staticcheck
  miss)

This is a single Go module, so `./...` from the repository root covers
everything including `internal/sockettest`.

CI builds on the two most recent Go releases, mirroring Go's own release
policy. Do not use language or library features newer than the minimum
version in `go.mod`.

## Platforms

- Cross-platform code lives in unsuffixed files such as `conn.go` and must
  build on Linux, macOS, and the BSDs. Prefer this location for a new API
  when the underlying system call exists everywhere.
- Linux-only code lives in `*_linux.go` files with a `//go:build linux`
  constraint. When a feature needs a stub elsewhere, pair it with a
  `*_others.go` file (see `netns_linux.go` and `netns_others.go`).
- Prefer the typed wrappers in `golang.org/x/sys/unix`. When one does not
  exist, call `unix.Syscall` or `unix.Syscall6` directly and keep any
  `unsafe.Pointer` to `uintptr` conversion inside the call expression so
  `go vet` accepts it.

## Code style

Blank lines:

- Leave an empty line after a closing brace and after a `var (` / `const (`
  block when another statement or declaration follows at the same indent
  level. Keep `}`, `)`, `case`, `default:`, and `else` continuations tight
  against what precedes them.
- Break dense bodies, especially tests, with blank lines at logical seams:
  between one actor's cluster of steps and the next, before final verdict
  assertions, and between constructing a fixture and the next setup
  statement.
- A single multi-line struct literal is one logical unit: never split it
  mid-literal.

Declaration layout:

- A type and all of its methods stay contiguous. Supporting enums and codes
  go before the type that uses them.
- `Conn` system call wrappers are one method per system call, named after
  it, ordered roughly alphabetically within their file. Typed variants share
  a prefix such as `GetsockoptInt` and `GetsockoptString`.
- Every `Conn` method that touches the file descriptor goes through
  `control`/`controlT` for non-blocking calls or `ReadFunc`/`WriteFunc`
  (and the generic `readT`/`writeT`) for calls that must wait on the
  runtime network poller. Never call `c.rc` directly from a new method, and
  never duplicate the retry, deadline, or cancelation logic in `rwT`.
- Closures passed to these helpers return the raw error from the system
  call. The helper decides readiness via `ready` and wraps the result with
  `os.NewSyscallError(op, err)`; callers must not wrap twice.
- Paired values described by one doc comment share one declaration. A doc
  comment attaches to the declaration, so separate lines leave every name
  after the first undocumented in go doc and IDE hover.
- Keep groups of one-line method stubs compact and aligned, with no blank
  lines between them.

Exported doc comments:

- Open with "X wraps y(2)" for a plain system call wrapper. For anything
  with more behavior, state the contract directly and self-contained: what
  the closure receives, what it must return, when it may be called again,
  and how errors are wrapped.
- Plain prose: short sentences, no em dashes. Prefer separate sentences, a
  colon, or "such as X or Y" over parenthetical asides. Internal comments
  are exempt.

Markdown documents:

- Wrap prose at 80 columns for terminal splits. Table rows and lines that
  carry a URL are exempt: they cannot wrap.

## Tests

- Never sleep in tests. Every awaited condition must be signaled; poll
  loops with sleep intervals count as sleeping. To prove a retry happened,
  have the closure signal a channel on its first attempt and wait on it
  before acting on the peer.
- Tests live in the external `socket_test` package. Use
  `internal/sockettest` for TCP fixtures backed by `*socket.Conn`, and
  `unix.Socketpair` wrapped by `socket.New` when a test only needs two
  connected descriptors.
- Independent scenarios are individual top-level Test functions. `t.Run`
  is for a table's cases and for subtests sharing a fixture built by the
  parent.
- Test scenarios, not coverage. Cover paths a plausible real-world scenario
  hits, framed on behavior; 100% coverage is not a goal. For blocking
  operations that means success, readiness retry, context cancel, context
  deadline plus its disarm for the next call, and `EBADF` after `Close`.
- Linux-only tests go in `*_linux_test.go`. Skip, rather than fail, when a
  test needs privileges the environment lacks, such as a network namespace.
- Test helpers, rig types, and shared fixtures go at the end of test files,
  after every Test/Fuzz/Benchmark/Example function. Shared consts may stay
  at the top.

## Changelog and releases

- Every user-visible change gets a bullet in `CHANGELOG.md` under an
  `## Unreleased` heading at the top, tagged `[New API]`, `[Bug Fix]`, or
  `[Improvement]`, with a PR link when one exists.
- Do not bump versions or create tags as part of a change; releases are cut
  separately.
