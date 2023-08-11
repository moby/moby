package container

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

// TestContainerConfig holds container configuration struct that
// are used in api calls.
type TestContainerConfig struct {
	Name             string
	Config           *container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
	Platform         *ocispec.Platform
}

// NewTestConfig creates a new TestContainerConfig with the provided options.
//
// If no options are passed, it creates a default config, which is a busybox
// container running "top" (on Linux) or "sleep" (on Windows).
func NewTestConfig(ops ...func(*TestContainerConfig)) *TestContainerConfig {
	cmd := []string{"top"}
	if runtime.GOOS == "windows" {
		cmd = []string{"sleep", "240"}
	}
	config := &TestContainerConfig{
		Config: &container.Config{
			Image: "busybox",
			Cmd:   cmd,
		},
		HostConfig:       &container.HostConfig{},
		NetworkingConfig: &network.NetworkingConfig{},
	}

	for _, op := range ops {
		op(config)
	}

	return config
}

// Create creates a container with the specified options, asserting that there was no error.
func Create(ctx context.Context, t *testing.T, apiClient client.APIClient, ops ...func(*TestContainerConfig)) string {
	t.Helper()
	config := NewTestConfig(ops...)
	c, err := apiClient.ContainerCreate(ctx, config.Config, config.HostConfig, config.NetworkingConfig, config.Platform, config.Name)
	assert.NilError(t, err)

	return c.ID
}

// CreateFromConfig creates a container from the given TestContainerConfig.
//
// Example use:
//
//	ctr, err := container.CreateFromConfig(ctx, apiClient, container.NewTestConfig(container.WithAutoRemove))
//	assert.Check(t, err)
func CreateFromConfig(ctx context.Context, apiClient client.APIClient, config *TestContainerConfig) (container.CreateResponse, error) {
	return apiClient.ContainerCreate(ctx, config.Config, config.HostConfig, config.NetworkingConfig, config.Platform, config.Name)
}

// Run creates and start a container with the specified options
func Run(ctx context.Context, t *testing.T, apiClient client.APIClient, ops ...func(*TestContainerConfig)) string {
	t.Helper()
	id := Create(ctx, t, apiClient, ops...)

	err := apiClient.ContainerStart(ctx, id, types.ContainerStartOptions{})
	assert.NilError(t, err)

	return id
}

type RunResult struct {
	ContainerID string
	ExitCode    int
	Stdout      *bytes.Buffer
	Stderr      *bytes.Buffer
}

func RunAttach(ctx context.Context, t *testing.T, apiClient client.APIClient, ops ...func(config *TestContainerConfig)) RunResult {
	t.Helper()

	ops = append(ops, func(c *TestContainerConfig) {
		c.Config.AttachStdout = true
		c.Config.AttachStderr = true
	})
	id := Create(ctx, t, apiClient, ops...)

	aresp, err := apiClient.ContainerAttach(ctx, id, types.ContainerAttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	assert.NilError(t, err)

	err = apiClient.ContainerStart(ctx, id, types.ContainerStartOptions{})
	assert.NilError(t, err)

	s, err := demultiplexStreams(ctx, aresp)
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		assert.NilError(t, err)
	}

	// Inspect to get the exit code. A new context is used here to make sure that if the context passed as argument as
	// reached timeout during the demultiplexStream call, we still return a RunResult.
	resp, err := apiClient.ContainerInspect(context.Background(), id)
	assert.NilError(t, err)

	return RunResult{ContainerID: id, ExitCode: resp.State.ExitCode, Stdout: &s.stdout, Stderr: &s.stderr}
}

type streams struct {
	stdout, stderr bytes.Buffer
}

// demultiplexStreams starts a goroutine to demultiplex stdout and stderr from the types.HijackedResponse resp and
// waits until either multiplexed stream reaches EOF or the context expires. It unconditionally closes resp and waits
// until the demultiplexing goroutine has finished its work before returning.
func demultiplexStreams(ctx context.Context, resp types.HijackedResponse) (streams, error) {
	var s streams
	outputDone := make(chan error, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		_, err := stdcopy.StdCopy(&s.stdout, &s.stderr, resp.Reader)
		outputDone <- err
		wg.Done()
	}()

	var err error
	select {
	case copyErr := <-outputDone:
		err = copyErr
		break
	case <-ctx.Done():
		err = ctx.Err()
	}

	resp.Close()
	wg.Wait()
	return s, err
}
