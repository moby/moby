package client_test

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"time"

	"github.com/moby/moby/client"
)

func ExampleClient_ContainerLogs() {
	apiClient, err := client.New(
		client.FromEnv,
		client.WithUserAgent("my-application/1.0.0"),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := apiClient.ContainerLogs(ctx, "my_container_id", client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer res.Close()

	_, err = io.Copy(os.Stdout, res)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Fatal(err)
	}
}
