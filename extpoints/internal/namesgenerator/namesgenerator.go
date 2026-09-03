// Package namesgenerator implements behavior shared by name-generator points.
package namesgenerator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moby/extensions"
)

// Generate calls the effective provider for a name-generator point.
// A replacement hides the built-in during normal resolution, but the full
// provider list retains it as a fallback.
// If the replacement fails and the fallback succeeds, Generate returns the
// fallback name together with the replacement error so callers can report the
// extension failure without blocking object creation.
// If the fallback also fails, the returned error contains both failures.
func Generate(ctx context.Context, resolver extensions.Resolver, point extensions.PointID, invoke func(context.Context, any) (string, error)) (string, error) {
	providers := resolver.Providers(point)
	primary, err := effectiveProvider(point, providers)
	if err != nil {
		return "", err
	}

	name, primaryErr := generate(ctx, point, primary, invoke)
	if primaryErr == nil || primary.Identity.Origin.Kind == extensions.ExtensionOriginBuiltin {
		return name, primaryErr
	}

	fallback, ok := findBuiltinFallback(providers, primary)
	if !ok {
		return "", primaryErr
	}

	name, fallbackErr := generate(ctx, point, fallback, invoke)
	return name, errors.Join(primaryErr, fallbackErr)
}

func effectiveProvider(point extensions.PointID, providers []extensions.ResolvedProvider) (extensions.ResolvedProvider, error) {
	effective := extensions.EffectiveProviders(providers)
	switch len(effective) {
	case 0:
		return extensions.ResolvedProvider{}, fmt.Errorf("point %q has no providers", point)
	case 1:
		return effective[0], nil
	default:
		return extensions.ResolvedProvider{}, fmt.Errorf("point %q has multiple providers", point)
	}
}

func findBuiltinFallback(providers []extensions.ResolvedProvider, primary extensions.ResolvedProvider) (extensions.ResolvedProvider, bool) {
	for _, provider := range providers {
		if provider.Identity.Origin.Kind == extensions.ExtensionOriginBuiltin && provider.Identity.ID != primary.Identity.ID {
			return provider, true
		}
	}
	return extensions.ResolvedProvider{}, false
}

func generate(ctx context.Context, point extensions.PointID, provider extensions.ResolvedProvider, invoke func(context.Context, any) (string, error)) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	name, err := invoke(ctx, provider.Impl)
	if err != nil {
		return "", providerError(point, provider.Identity.ID, err)
	}
	if name == "" {
		return "", providerError(point, provider.Identity.ID, errors.New("provider returned an empty name"))
	}
	if !isValidGeneratedName(name) {
		return "", providerError(point, provider.Identity.ID, errors.New(
			"provider returned an invalid name: expected 2-63 ASCII letters, digits, hyphens, or underscores, starting and ending with a letter or digit",
		))
	}
	return name, nil
}

func isValidGeneratedName(name string) bool {
	if len(name) < 2 || len(name) > 63 {
		return false
	}
	for i := range len(name) {
		ch := name[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		if i > 0 && i < len(name)-1 && (ch == '-' || ch == '_') {
			continue
		}
		return false
	}
	return true
}

func providerError(point extensions.PointID, extension extensions.ExtensionID, err error) error {
	return fmt.Errorf("%s provider %q: %w", point.Name(), extension, err)
}
