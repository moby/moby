package extensions

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
)

// ExtensionID identifies a deployable extension.
type ExtensionID string

// PointID identifies an extension point contract.
type PointID string

// Dependency declares one extension dependency.
type Dependency struct {
	Point     PointID
	Extension ExtensionID
	Optional  bool
}

// Provider is an extension's in-process implementation of one point, as
// declared. Impl stores the implementation behind typed point handles. It
// carries no extension id: which extension declared it is known from the
// declaration it belongs to, and the broker reports it as the
// [ResolvedProvider.Extension] of a lookup result.
type Provider struct {
	Point PointID
	Impl  any
}

// ResolvedProvider is a provider as returned from a lookup: the same
// implementation, plus the id of the extension that provides it. It is the
// output counterpart to the declaration-side [Provider].
type ResolvedProvider struct {
	Extension ExtensionID
	Impl      any
}

// TypedProvider is a provider returned through a typed point handle.
type TypedProvider[T any] struct {
	Extension ExtensionID
	Impl      T
}

// Point binds a point ID to the Go interface implemented by its providers.
type Point[T any] struct {
	id PointID
}

// pointIDPattern is the required shape of a point id: a reverse-DNS-style,
// dot-separated name of at least three segments ending in a version, i.e.
// <tld>.<name>...vN (e.g. org.mobyproject.extension.volume.driver.v1). Segments
// are lowercase and may contain digits, hyphens, and underscores; the last is a
// version like v0 or v12.
var pointIDPattern = lazyRegexp(`^[a-z][a-z0-9]*(\.[a-z0-9_-]+)+\.v[0-9]+$`)

// DefinePoint binds id to provider interface T. It panics if id is not a valid
// point id (see [pointIDPattern]): a namespaced, versioned name such as
// org.mobyproject.extension.volume.driver.v1. The id is fixed in source, so a
// malformed one is a programming error worth catching at startup rather than
// when some extension first names the point -- the same rationale as compiling
// a constant regexp up front.
func DefinePoint[T any](id PointID) Point[T] {
	if !pointIDPattern().MatchString(string(id)) {
		panic(fmt.Sprintf("extensions: invalid point id %q: want <tld>.<name>...vN, e.g. org.mobyproject.extension.volume.driver.v1", id))
	}
	return Point[T]{id: id}
}

// extensionIDPattern is the required shape of an extension id: a reverse-DNS
// name of at least two lowercase, dot-separated segments followed by a
// mandatory version segment (e.g. org.example.no-privileged.v1 or
// com.docker.compose.v1). Segments are lowercase alphanumerics and may contain
// hyphens but not lead or trail with one; the final segment is a version like v0
// or v12. That version is a namespace element, not a semantic version, and is
// distinct from the versions of the points the extension implements: migrating
// com.foo.v1 to com.foo.v2 is a new extension -- a different id, binary, and
// config -- not an upgrade in place; the two can coexist during migration.
// Because an id is also a config key, a dependency name, and the on-disk binary
// name, this shape doubles as a safety rule: it admits no path separators, no
// "..", no uppercase, and no other path- or shell-hostile characters.
var extensionIDPattern = lazyRegexp(`^[a-z0-9]+(-[a-z0-9]+)*(\.[a-z0-9]+(-[a-z0-9]+)*)+\.v[0-9]+$`)

// ValidateExtensionID reports whether id is a well-formed extension id (see
// [extensionIDPattern]). It is enforced where an extension is registered, so a
// malformed id -- in-process or delivered by a launched binary -- is rejected
// rather than used as a binary name or config key.
func ValidateExtensionID(id ExtensionID) error {
	if id == "" {
		return errors.New("extension id is required")
	}
	if !extensionIDPattern().MatchString(string(id)) {
		return fmt.Errorf("invalid extension id %q: want a versioned reverse-DNS name like org.example.myext.v1, lowercase, no path-hostile characters", id)
	}
	return nil
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

// lazyRegexp returns a regexp accessor that compiles pattern on first use.
// The daemon/internal/lazyregexp helper is not importable from this package.
func lazyRegexp(pattern string) func() *regexp.Regexp {
	return sync.OnceValue(func() *regexp.Regexp {
		re, err := regexp.Compile(pattern)
		if err != nil {
			panic(err)
		}
		return re
	})
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

// Single returns the only point provider.
func (p Point[T]) Single(r Resolver) (T, error) {
	provider, err := r.SingleProvider(p.id)
	if err != nil {
		var zero T
		return zero, err
	}
	return typedProvider[T](p.id, "", provider)
}

// All returns all point providers.
func (p Point[T]) All(r Resolver) ([]TypedProvider[T], error) {
	providers := r.Providers(p.id)
	typed := make([]TypedProvider[T], 0, len(providers))
	for _, provider := range providers {
		impl, err := typedProvider[T](p.id, provider.Extension, provider.Impl)
		if err != nil {
			return nil, err
		}
		typed = append(typed, TypedProvider[T]{Extension: provider.Extension, Impl: impl})
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

// Resolver exposes provider lookup to extension initializers.
type Resolver interface {
	Provider(PointID, ExtensionID) (any, error)
	SingleProvider(PointID) (any, error)
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

// Config is an extension's per-extension configuration, delivered by id: to
// in-process extensions through Init, and to out-of-process ones through the
// startup handshake. It is the parsed configuration object (as from
// daemon.json), so an extension reads its keys directly or decodes it into a
// struct.
type Config = map[string]any

// Extension is something a host runs: it declares itself. A stateless extension
// is a [Declaration] wrapped with [New]; an extension that holds state
// implements this interface on its own type, so the object that implements the
// point also configures itself from its config in Init -- no package-level
// state needed.
type Extension interface {
	// Declaration returns the extension's id, providers, dependencies, and
	// conflicts, plus its optional Init and Shutdown.
	Declaration() Declaration
}

// Declaration declares one extension and its dependencies.
// Conflicts names extensions that cannot coexist with this extension. Init, if
// set, configures the extension from the config the host delivers; Shutdown
// tears it down.
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
