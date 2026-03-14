//nolint:staticcheck // Uses deprecated functions on purpose
package main

import "github.com/moby/moby/client"

// KO fails (due to nested inline?)
func KO() {
	_, _ = client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.38"))
}
