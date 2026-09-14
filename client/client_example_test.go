package client_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

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

// ExampleWithHTTPRequestHook demonstrates using a request hook to add
// metadata that varies between requests made by the same client.
func ExampleWithHTTPRequestHook() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("X-Request-ID:", r.Header.Get("X-Request-ID"))
	}))
	defer server.Close()

	type requestIDKey struct{}
	apiClient, err := client.New(
		client.WithHost(server.URL),
		client.WithHTTPRequestHook(func(req *http.Request) error {
			if requestID, ok := req.Context().Value(requestIDKey{}).(string); ok {
				req.Header.Set("X-Request-ID", requestID)
			}
			return nil
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer apiClient.Close()

	// Use the same client with different request-scoped metadata.
	for _, requestID := range []string{"request-1", "request-2"} {
		ctx := context.WithValue(context.Background(), requestIDKey{}, requestID)
		if _, err := apiClient.Ping(ctx, client.PingOptions{}); err != nil {
			log.Fatal(err)
		}
	}

	// Output:
	// X-Request-ID: request-1
	// X-Request-ID: request-2
}
