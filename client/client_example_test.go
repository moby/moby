package client_test

import (
	"context"
	"fmt"
	"log"

	"github.com/moby/moby/client"
)

func ExampleNew() {
	// Create a new client that handles common environment variables
	// for configuration (DOCKER_HOST, DOCKER_API_VERSION), and does
	// API-version negotiation to allow downgrading the API version
	// when connecting with an older daemon version.
	apiClient, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}

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
