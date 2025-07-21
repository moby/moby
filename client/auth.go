package client

import (
	"context"

	"github.com/moby/moby/api/types/registry"
)

// staticAuth creates a privilegeFn from the given registryAuth.
func staticAuth(registryAuth string) registry.RequestAuthConfig {
	return func(ctx context.Context) (string, error) {
		return registryAuth, nil
	}
}
