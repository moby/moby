//nolint:staticcheck // Uses deprecated functions on purpose
package main

import (
	"context"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/strslice"
	"github.com/moby/moby/client"
)

func OK() {
	opts := []client.Opt{
		client.FromEnv,
		client.WithVersion("1.38"),
		client.WithVersionFromEnv(),
	}

	c, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return
	}

	_, _ = c.ContainerCreate(context.Background(), client.ContainerCreateOptions{
		Config: &container.Config{
			Image: "busybox",
			Cmd:   strslice.StrSlice{"top"},
		},
	})
	_ = c.Close()
}
