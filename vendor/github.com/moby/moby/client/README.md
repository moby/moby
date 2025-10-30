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
	apiClient, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(context.Background(), client.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	for _, ctr := range containers {
		fmt.Printf("%s %s (status: %s)\n", ctr.ID, ctr.Image, ctr.Status)
	}
}
```

[Full documentation is available on pkg.go.dev.](https://pkg.go.dev/github.com/moby/moby/client)
