//nolint:staticcheck // Uses deprecated functions on purpose
package main

import "github.com/moby/moby/client"

func OK() {
	_, _ = client.New(client.FromEnv, client.WithVersion("1.38"), client.WithVersionFromEnv())
}
