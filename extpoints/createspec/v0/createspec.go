package createspecv0

import (
	"context"
	"fmt"
	"time"

	"github.com/moby/moby/v2/internal/extensions"
)

// Hook is the create-spec hook provider interface. It fires when the daemon has
// generated a container's OCI runtime spec (at start, on the fully-formed
// container), so a provider sees the real container id and the complete spec --
// namespaces, capabilities, seccomp, devices, mounts, resources. A provider may
// reshape the spec in CreateSpec and veto the start in Validate.
//
// This is the right altitude for two kinds of extension: a runtime-config
// bridge such as NRI, which adjusts the OCI spec the way containerd's NRI does,
// and a security extension hardening or auditing what will actually run. The
// container-create hook, by contrast, sees only the create request before the
// container is formed.
//
// Failure semantics are fail-closed, because this is a security-relevant veto
// point: a provider that returns an error aborts the container start. Each call
// is bounded by a 30s deadline; an out-of-process provider that stops answering
// hits that deadline over gRPC and is treated as a veto, while an in-process
// provider is trusted to honor its context (the deadline is passed to it but not
// forced, since abandoning a direct Go call on a goroutine would race on the
// shared spec). So the timeout is only enforced for out-of-process providers: an
// in-process provider that ignores it can stall the start, and honoring ctx is
// part of the contract. Providers are called in sequence, each bounded
// independently, so one slow provider cannot silently be skipped.
//
// It is plain Go: in-process providers implement it directly, and out-of-process
// providers are reached through an adapter that satisfies it (see
// [ClientProvider]).
type Hook interface {
	// CreateSpec returns the modified OCI runtime spec, or nil to leave it
	// unchanged. Providers run in sequence, each receiving the spec as shaped by
	// the providers before it -- mirroring how the daemon itself builds the spec
	// through a chain of mutating options.
	CreateSpec(ctx context.Context, req *SpecRequest) (*SpecAdjustment, error)
	// Validate inspects the final spec and vetoes the start by returning an error.
	Validate(ctx context.Context, req *SpecRequest) error
}

// SpecRequest carries the container's OCI runtime spec and identity to a hook.
// The pb struct tags are the proto field numbers -- the source of truth for wire
// compatibility, and what the generator reads to emit the .proto and conversions.
type SpecRequest struct {
	ContainerID string `pb:"1"`
	Name        string `pb:"2"`
	// Spec is the OCI runtime spec as canonical runtime-spec JSON. It is passed
	// verbatim rather than re-modeled in proto, so a provider works against the
	// actual, standard schema in any language.
	Spec   []byte            `pb:"3"`
	Labels map[string]string `pb:"4"`
}

// SpecAdjustment is a provider's reshaped spec.
type SpecAdjustment struct {
	// Spec is the modified OCI runtime spec as JSON, or empty to leave the spec
	// unchanged. Unlike the create hook's additive patch, a spec mutator returns
	// the whole spec, because providers run in sequence rather than independently.
	Spec []byte `pb:"1"`
}

// Point is the create-spec hook point.
var Point = extensions.DefinePoint[Hook]("org.mobyproject.extension.container.create_spec.v0")

// Enabled reports whether any provider implements the point. A caller uses it to
// skip building the request -- notably marshaling the whole OCI spec -- on every
// container start when no extension is registered for the point.
func Enabled(resolver extensions.Resolver) (bool, error) {
	hooks, err := Point.All(resolver)
	if err != nil {
		return false, err
	}
	return len(hooks) > 0, nil
}

// CreateSpec threads the spec through every provider in turn and returns the
// final spec JSON. Spec mutation is sequential, not an order-independent merge:
// a provider sees the spec as adjusted by those applied before it. The fan-out
// order is unspecified, so with more than one provider the composition order is
// undefined -- acceptable while providers touch disjoint parts of the spec, and
// moot today with a single provider (the NRI bridge, which orders its own
// plugins internally). A per-point priority mechanism could impose a
// deterministic order later.
func CreateSpec(ctx context.Context, resolver extensions.Resolver, req *SpecRequest) ([]byte, error) {
	hooks, err := Point.All(resolver)
	if err != nil {
		return nil, err
	}
	spec := req.Spec
	for _, hook := range hooks {
		cur := *req
		cur.Spec = spec
		callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		adj, err := hook.Impl.CreateSpec(callCtx, &cur)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("create_spec provider %q: %w", hook.Extension, err)
		}
		if adj != nil && len(adj.Spec) > 0 {
			spec = adj.Spec
		}
	}
	return spec, nil
}

// Validate calls every provider with the final spec. A provider returning an
// error -- or not answering within the 30s deadline -- vetoes the container
// start (fail-closed).
func Validate(ctx context.Context, resolver extensions.Resolver, req *SpecRequest) error {
	hooks, err := Point.All(resolver)
	if err != nil {
		return err
	}
	for _, hook := range hooks {
		callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := hook.Impl.Validate(callCtx, req)
		cancel()
		if err != nil {
			return fmt.Errorf("create_spec provider %q vetoed the start: %w", hook.Extension, err)
		}
	}
	return nil
}
