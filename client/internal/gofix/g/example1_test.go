//nolint:staticcheck // Uses deprecated functions on purpose
package main

import "github.com/moby/moby/client"

// OK works when in a separate package (see package "b"), but fails when in same package as [KO]
func OK() {
	_, _ = client.New(client.FromEnv, client.WithVersion("1.38"))
}
