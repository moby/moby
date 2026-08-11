//go:generate mobyextgen

// Package namesgeneratorv0 defines the experimental point for generating
// default object names.
package namesgeneratorv0

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moby/extensions"
)

// NamesGenerator generates a default name.
type NamesGenerator interface {
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateReply, error)
}

// GenerateRequest describes a name generation attempt.
type GenerateRequest struct {
	// Retry is the number of prior generated names that collided.
	Retry int64 `pb:"1"`
}

// GenerateReply contains a generated name.
type GenerateReply struct {
	Name string `pb:"1"`
}

// Point is the single-provider name generator point.
//
//mobyextgen:service=NamesGenerator
var Point = extensions.DefineSinglePoint[NamesGenerator]("org.mobyproject.extension.namesgenerator.v0")

// Generate calls the effective provider for the names generator point.
// A replacement hides the built-in during normal resolution, but the full
// provider list retains it as a fallback.
// If the replacement fails and the fallback succeeds, Generate returns the
// fallback reply together with the replacement error so callers can report the
// extension failure without blocking object creation.
// If the fallback also fails, the returned error contains both failures.
func Generate(ctx context.Context, resolver extensions.Resolver, req *GenerateRequest) (*GenerateReply, error) {
	providers := resolver.Providers(Point.ID())
	primary, err := effectiveProvider(providers)
	if err != nil {
		return nil, err
	}

	reply, primaryErr := generate(ctx, primary, req)
	if primaryErr == nil || primary.Builtin {
		return reply, primaryErr
	}

	fallback, ok := findBuiltinFallback(providers, primary)
	if !ok {
		return nil, primaryErr
	}

	reply, fallbackErr := generate(ctx, fallback, req)
	return reply, errors.Join(primaryErr, fallbackErr)
}

func effectiveProvider(providers []extensions.ResolvedProvider) (extensions.ResolvedProvider, error) {
	effective := extensions.EffectiveProviders(providers)
	switch len(effective) {
	case 0:
		return extensions.ResolvedProvider{}, fmt.Errorf("point %q has no providers", Point.ID())
	case 1:
		return effective[0], nil
	default:
		return extensions.ResolvedProvider{}, fmt.Errorf("point %q has multiple providers", Point.ID())
	}
}

func findBuiltinFallback(providers []extensions.ResolvedProvider, primary extensions.ResolvedProvider) (extensions.ResolvedProvider, bool) {
	for _, provider := range providers {
		if provider.Builtin && provider.Extension != primary.Extension {
			return provider, true
		}
	}
	return extensions.ResolvedProvider{}, false
}

// generate makes one bounded provider attempt and validates its reply.
func generate(ctx context.Context, provider extensions.ResolvedProvider, req *GenerateRequest) (*GenerateReply, error) {
	generator, ok := provider.Impl.(NamesGenerator)
	if !ok {
		return nil, fmt.Errorf("extension %q provider for point %q has type %T", provider.Extension, Point.ID(), provider.Impl)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	reply, err := generator.Generate(ctx, req)
	if err != nil {
		return nil, providerError(provider.Extension, err)
	}
	if reply == nil {
		return nil, providerError(provider.Extension, errors.New("provider returned a nil reply"))
	}
	if reply.Name == "" {
		return nil, providerError(provider.Extension, errors.New("provider returned an empty name"))
	}
	return reply, nil
}

func providerError(extension extensions.ExtensionID, err error) error {
	return fmt.Errorf("%s provider %q: %w", Point.ID().Name(), extension, err)
}
