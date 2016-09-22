## Client

The client package implements a fully featured http client to interact with the Docker engine. It's modeled after the requirements of the Docker engine CLI, but it can also serve other purposes.

### Usage

You can use this client package in your applications by creating a new client object. Then use that object to execute operations against the remote server. Follow the example below to see how to list all the containers running in a Docker engine host:

```go
package main

import (
	"fmt"
	"context"

	"github.com/docker/docker/client"
	"github.com/docker/docker/api/types"
)

func main() {
	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	cli, err := client.NewClient("unix:///var/run/docker.sock", "v1.22", nil, defaultHeaders)
	if err != nil {
		panic(err)
	}

	options := types.ContainerListOptions{All: true}
	containers, err := cli.ContainerList(context.Background(), options)
	if err != nil {
		panic(err)
	}

	for _, c := range containers {
		fmt.Println(c.ID)
	}
}
```
