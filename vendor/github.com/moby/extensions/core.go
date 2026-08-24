package extensions

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// ExtensionID identifies a deployable extension.
type ExtensionID string

// ExtensionOriginKind identifies how the host obtained an extension.
type ExtensionOriginKind string

const (
	// ExtensionOriginBuiltin identifies an extension compiled into the host.
	ExtensionOriginBuiltin ExtensionOriginKind = "builtin"
	// ExtensionOriginExecutable identifies an extension launched as an
	// executable.
	ExtensionOriginExecutable ExtensionOriginKind = "executable"
)

// ExtensionOrigin identifies how the host obtained an extension and its
// kind-specific attested details.
type ExtensionOrigin struct {
	Kind       ExtensionOriginKind
	Executable *ExecutableOrigin
}

// ExecutableOrigin contains host-attested details for an executable origin.
type ExecutableOrigin struct {
	// Path is the exact path the host executed, as discovered from the
	// extension directory. It is not normalized: a relative directory or a
	// symlink yields that relative or symlink path.
	Path string
}

// ExtensionIdentity is the host-attested identity of an extension.
// The extension declares ID, but the host supplies and validates the complete
// identity, including Origin.
type ExtensionIdentity struct {
	ID     ExtensionID
	Origin ExtensionOrigin
}

// ValidateExtensionIdentity returns an error unless identity contains a valid
// extension ID and a recognized host-attested origin.
func ValidateExtensionIdentity(identity ExtensionIdentity) error {
	if err := ValidateExtensionID(identity.ID); err != nil {
		return err
	}
	switch identity.Origin.Kind {
	case "":
		return errors.New("extension origin kind is required")
	case ExtensionOriginBuiltin:
		if identity.Origin.Executable != nil {
			return errors.New("builtin extension origin must not include executable payload")
		}
	case ExtensionOriginExecutable:
		if identity.Origin.Executable == nil {
			return errors.New("executable extension origin requires executable payload")
		}
		if identity.Origin.Executable.Path == "" {
			return errors.New("executable origin path is required")
		}
	default:
		return fmt.Errorf("invalid extension origin kind %q: want %q or %q", identity.Origin.Kind, ExtensionOriginBuiltin, ExtensionOriginExecutable)
	}
	return nil
}

// PointID identifies an extension point contract.
type PointID string

// IsMetadataPoint reports whether point carries framework metadata rather than
// a callable capability.
// Host provider admission skips metadata points; the servicev0 marker controls
// publication.
// Currently the servicev0 offer marker is the only metadata point; its package
// tests keep this ID in sync with the point definition.
func IsMetadataPoint(point PointID) bool {
	return point == "org.mobyproject.extension.service.v0"
}

// Name returns the point's short name, the segment before its version.
func (id PointID) Name() string {
	segments := strings.Split(string(id), ".")
	if len(segments) < 2 {
		return string(id)
	}
	return segments[len(segments)-2]
}

// pointIDPattern requires a lowercase reverse-DNS-style id ending in a vN
// version. Segments may contain digits, hyphens, and underscores.
var pointIDPattern = lazyRegexp(`^[a-z][a-z0-9]*(\.[a-z0-9_-]+)+\.v[0-9]+$`)

// ValidatePointID reports whether id is a well-formed, versioned Point ID.
func ValidatePointID(id PointID) error {
	if id == "" {
		return errors.New("point id is required")
	}
	if !pointIDPattern().MatchString(string(id)) {
		return fmt.Errorf("invalid point id %q: want a versioned reverse-DNS name like org.example.api.v1", id)
	}
	return nil
}

// extensionIDPattern requires a lowercase reverse-DNS name with a mandatory vN
// version. The version is a namespace element, not a semantic version: changing
// com.foo.v1 to com.foo.v2 creates a new extension, binary, and configuration.
// The restricted characters also make the id safe as a config key and filename.
var extensionIDPattern = lazyRegexp(`^[a-z0-9]+(-[a-z0-9]+)*(\.[a-z0-9]+(-[a-z0-9]+)*)+\.v[0-9]+$`)

// ValidateExtensionID reports whether id is well-formed before it is used as a
// configuration key or binary name.
func ValidateExtensionID(id ExtensionID) error {
	if id == "" {
		return errors.New("extension id is required")
	}
	if !extensionIDPattern().MatchString(string(id)) {
		return fmt.Errorf("invalid extension id %q: want a versioned reverse-DNS name like org.example.myext.v1, lowercase, no path-hostile characters", id)
	}
	return nil
}

func lazyRegexp(pattern string) func() *regexp.Regexp {
	return sync.OnceValue(func() *regexp.Regexp {
		re, err := regexp.Compile(pattern)
		if err != nil {
			panic(err)
		}
		return re
	})
}

// Dependency declares one extension dependency.
type Dependency struct {
	Point     PointID
	Extension ExtensionID
	Optional  bool
}

// Provider declares an in-process implementation for a point.
type Provider struct {
	Point PointID
	Impl  any
}

// ResolvedProvider is a provider returned from a lookup with its host-attested
// extension identity.
type ResolvedProvider struct {
	Identity ExtensionIdentity
	Impl     any
}

// EffectiveProviders applies origin precedence for single-provider resolution.
// Built-ins are used only when no executable provider exists.
// Fan-out and by-id lookups do not apply this precedence.
func EffectiveProviders(providers []ResolvedProvider) []ResolvedProvider {
	var executables, builtins []ResolvedProvider
	for _, p := range providers {
		if p.Identity.Origin.Kind == ExtensionOriginBuiltin {
			builtins = append(builtins, p)
		} else {
			executables = append(executables, p)
		}
	}
	if len(executables) == 0 {
		return builtins
	}
	return executables
}

// TypedProvider is a provider returned through a typed point handle.
type TypedProvider[T any] struct {
	Identity ExtensionIdentity
	Impl     T
}

// Point binds a point ID to the Go interface implemented by its providers.
type Point[T any] struct {
	id PointID
}

// DefinePoint binds id to provider interface T and panics for an invalid point
// id. Point ids are fixed in source, so invalid ids are programming errors.
func DefinePoint[T any](id PointID) Point[T] {
	if ValidatePointID(id) != nil {
		panic(fmt.Sprintf("extensions: invalid point id %q: want <tld>.<name>...vN, e.g. org.mobyproject.extension.volume.driver.v1", id))
	}
	return Point[T]{id: id}
}

// DefineSinglePoint declares a point with one deciding provider instead of a
// fan-out. The wire generator carries this cardinality into ClientPoint so the
// host can reject multiple executable providers at startup.
func DefineSinglePoint[T any](id PointID) Point[T] {
	return DefinePoint[T](id)
}

// ID returns the point identifier.
func (p Point[T]) ID() PointID {
	return p.id
}

// Provide returns a provider declaration for impl.
func (p Point[T]) Provide(impl T) Provider {
	return Provider{Point: p.id, Impl: impl}
}

// Dependency returns a required dependency declaration for the point: at least
// one provider must exist before the dependent initializes.
func (p Point[T]) Dependency() Dependency {
	return Dependency{Point: p.id}
}

// OptionalDependency returns an optional dependency declaration for the point:
// the dependent still initializes, ordered after any providers, when none exist.
func (p Point[T]) OptionalDependency() Dependency {
	return Dependency{Point: p.id, Optional: true}
}

// ByExtension returns the point provider implemented by extension.
func (p Point[T]) ByExtension(r Resolver, extension ExtensionID) (T, error) {
	provider, err := r.Provider(p.id, extension)
	if err != nil {
		var zero T
		return zero, err
	}
	return typedProvider[T](p.id, extension, provider)
}

// Single returns the only point provider, after origin precedence
// ([EffectiveProviders]): a built-in provider does not count against the
// one-provider limit, it stands in when no executable provider exists.
func (p Point[T]) Single(r Resolver) (T, error) {
	providers := EffectiveProviders(r.Providers(p.id))
	var zero T
	switch len(providers) {
	case 0:
		return zero, fmt.Errorf("point %q has no providers", p.id)
	case 1:
		return typedProvider[T](p.id, providers[0].Identity.ID, providers[0].Impl)
	default:
		return zero, fmt.Errorf("point %q has multiple providers", p.id)
	}
}

// Enabled reports whether any extension provides the point.
func (p Point[T]) Enabled(r Resolver) bool {
	return len(r.Providers(p.id)) > 0
}

// All returns all point providers, including built-in and executable providers.
func (p Point[T]) All(r Resolver) ([]TypedProvider[T], error) {
	providers := r.Providers(p.id)
	typed := make([]TypedProvider[T], 0, len(providers))
	for _, provider := range providers {
		impl, err := typedProvider[T](p.id, provider.Identity.ID, provider.Impl)
		if err != nil {
			return nil, err
		}
		typed = append(typed, TypedProvider[T]{Identity: provider.Identity, Impl: impl})
	}
	return typed, nil
}

func typedProvider[T any](point PointID, extension ExtensionID, provider any) (T, error) {
	typed, ok := provider.(T)
	if ok {
		return typed, nil
	}
	var zero T
	if extension == "" {
		return zero, fmt.Errorf("point %q provider has type %T", point, provider)
	}
	return zero, fmt.Errorf("extension %q provider for point %q has type %T", extension, point, provider)
}

// Resolver exposes provider lookup to extension initializers. Selection and
// cardinality are handled by the typed [Point] accessors.
type Resolver interface {
	Provider(PointID, ExtensionID) (any, error)
	Providers(PointID) []ResolvedProvider
}

// Registrar registers extensions.
type Registrar interface {
	Register(Extension) error
}

// RegisterAll registers exts with registrar.
func RegisterAll(registrar Registrar, exts ...Extension) error {
	for _, ext := range exts {
		if err := registrar.Register(ext); err != nil {
			return err
		}
	}
	return nil
}

// Config is parsed per-extension configuration delivered by id to Init or the
// out-of-process startup handshake.
type Config = map[string]any

// Extension is a host-managed extension declaration.
type Extension interface {
	// Declaration returns the extension's id, providers, dependencies, and
	// conflicts, plus its optional Init and Shutdown.
	Declaration() Declaration
}

// Declaration declares an extension, its providers and dependencies, and its
// optional lifecycle callbacks.
type Declaration struct {
	ID           ExtensionID
	Providers    []Provider
	Dependencies []Dependency
	Conflicts    []ExtensionID
	Init         func(context.Context, Config, Resolver) error
	Shutdown     func(context.Context) error
}

// New wraps a static Declaration as an [Extension].
func New(d Declaration) Extension { return staticExtension{d} }

type staticExtension struct{ decl Declaration }

func (e staticExtension) Declaration() Declaration { return e.decl }
