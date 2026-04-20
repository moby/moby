package client_test

import (
	"context"
	"fmt"
	"log"

	"github.com/moby/moby/client"
)

// This example demonstrates basic usage of the API client.
//
// It creates a new client with [client.New] using [client.FromEnv] (configuring
// the client from commonly used environment variables such as DOCKER_HOST and
// DOCKER_API_VERSION) and sets a custom User-Agent using [client.WithUserAgent].
//
// API-version negotiation is enabled by default to allow downgrading
// the API version when connecting with an older daemon version.
//
// It then lists all containers (both stopped and running) similar to
// "docker ps --all".
func Example() {
	apiClient, err := client.New(
		client.FromEnv,
		client.WithUserAgent("my-application/1.0.0"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer apiClient.Close()

	// List all containers (both stopped and running).
	result, err := apiClient.ContainerList(context.Background(), client.ContainerListOptions{
		All: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Print each container's ID, status and the image it was created from.
	fmt.Printf("%s  %-22s  %s\n", "ID", "STATUS", "IMAGE")
	for _, ctr := range result.Items {
		fmt.Printf("%s  %-22s  %s\n", ctr.ID, ctr.Status, ctr.Image)
	}
}
