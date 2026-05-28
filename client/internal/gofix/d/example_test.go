//nolint:staticcheck // Uses deprecated functions on purpose
package main

import "github.com/moby/moby/client"

func OK() {
	_, _ = client.NewClientWithOpts(client.FromEnv)
}
