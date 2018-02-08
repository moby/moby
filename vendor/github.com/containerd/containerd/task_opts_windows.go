package containerd

import (
	"context"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// WithResources sets the provided resources on the spec for task updates
func WithResources(resources *specs.WindowsResources) UpdateTaskOpts {
	return func(ctx context.Context, client *Client, r *UpdateTaskInfo) error {
		r.Resources = resources
		return nil
	}
}
