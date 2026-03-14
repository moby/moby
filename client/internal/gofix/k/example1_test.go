//nolint:staticcheck // Uses deprecated functions on purpose
package main

import "github.com/moby/moby/client"

// OK works and is in a different package than [KO] (which is in "main_test")
func OK() {
	_, _ = client.New(client.FromEnv, client.WithVersion("1.38"))
}
