package client_test

import (
	"context"
	"log"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func ExampleClient_ContainerWait_withTimeout() {
	apiClient, err := client.New(
		client.FromEnv,
		client.WithUserAgent("my-application/1.0.0"),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wait := apiClient.ContainerWait(ctx, "my_container_id", client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})
	if err := <-wait.Error; err != nil {
		log.Fatal(err)
	}
}
