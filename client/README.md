# Go client for the Docker Engine API

[![PkgGoDev](https://pkg.go.dev/badge/github.com/moby/moby/client)](https://pkg.go.dev/github.com/moby/moby/client)
![GitHub License](https://img.shields.io/github/license/moby/moby)
[![Go Report Card](https://goreportcard.com/badge/github.com/moby/moby/client)](https://goreportcard.com/report/github.com/moby/moby/client)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/moby/moby/badge)](https://scorecard.dev/viewer/?uri=github.com/moby/moby)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/10989/badge)](https://www.bestpractices.dev/projects/10989)

The `docker` command uses this package to communicate with the daemon. It can
also be used by your own Go applications to do anything the command-line
interface does; running containers, pulling or pushing images, etc.

For example, to list all containers (the equivalent of `docker ps --all`):

```go
package main

import (
	"context"
	"fmt"

	"github.com/moby/moby/client"
)

func main() {
	// Create a new client that handles common environment variables
	// for configuration (DOCKER_HOST, DOCKER_API_VERSION), and does
	// API-version negotiation to allow downgrading the API version
	// when connecting with an older daemon version.
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	// List all containers (both stopped and running).
	result, err := apiClient.ContainerList(context.Background(), client.ContainerListOptions{
		All: true,
	})
	if err != nil {
		panic(err)
	}

	// Print each container's ID, status and the image it was created from.
	fmt.Printf("%s  %-22s  %s\n", "ID", "STATUS", "IMAGE")
	for _, ctr := range result.Items {
		fmt.Printf("%s  %-22s  %s\n", ctr.ID, ctr.Status, ctr.Image)
	}
}
```

[Full documentation is available on pkg.go.dev.](https://pkg.go.dev/github.com/moby/moby/client)
