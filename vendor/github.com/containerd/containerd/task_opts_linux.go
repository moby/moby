package containerd

import (
	"context"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// WithResources sets the provided resources for task updates
func WithResources(resources *specs.LinuxResources) UpdateTaskOpts {
	return func(ctx context.Context, client *Client, r *UpdateTaskInfo) error {
		r.Resources = resources
		return nil
	}
}
