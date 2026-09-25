//nolint:staticcheck // Uses deprecated functions on purpose
package main

import "github.com/moby/moby/client"

// KO fails (due to nested inlines?) but works if options are in a slice.
func KO() {
	_, _ = client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.38"),
		client.WithVersionFromEnv(),
	)
}
