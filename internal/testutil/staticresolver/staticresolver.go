// Package staticresolver provides a fixed implementation of extensions.Resolver.
package staticresolver

import (
	"fmt"

	"github.com/moby/extensions"
)

// New returns a resolver backed by providers.
func New(providers ...extensions.ResolvedProvider) extensions.Resolver {
	return resolver(providers)
}

type resolver []extensions.ResolvedProvider

func (r resolver) Provider(_ extensions.PointID, id extensions.ExtensionID) (any, error) {
	for _, provider := range r {
		if provider.Identity.ID == id {
			return provider.Impl, nil
		}
	}
	return nil, fmt.Errorf("no provider for extension %q", id)
}

func (r resolver) Providers(extensions.PointID) []extensions.ResolvedProvider {
	return r
}
