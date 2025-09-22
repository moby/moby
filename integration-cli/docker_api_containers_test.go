package main

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/moby/moby/client/containerstats"
	"github.com/moby/moby/v2/integration-cli/cli"
	"github.com/moby/moby/v2/internal/testutil"
	"gotest.tools/v3/assert"
)

func (s *DockerAPISuite) TestContainerAPIGetAll(c *testing.T) {
	startCount := getContainerCount(c)
	const name = "getall"
	cli.DockerCmd(c, "run", "--name", name, "busybox", "true")

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	ctx := testutil.GetContext(c)
	containers, err := apiClient.ContainerList(ctx, client.ContainerListOptions{
		All: true,
	})
	assert.NilError(c, err)
	assert.Equal(c, len(containers), startCount+1)
	actual := containers[0].Names[0]
	assert.Equal(c, actual, "/"+name)
}

// regression test for empty json field being omitted #13691
func (s *DockerAPISuite) TestContainerAPIGetJSONNoFieldsOmitted(c *testing.T) {
	startCount := getContainerCount(c)
	cli.DockerCmd(c, "run", "busybox", "true")

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	options := client.ContainerListOptions{
		All: true,
	}
	ctx := testutil.GetContext(c)
	containers, err := apiClient.ContainerList(ctx, options)
	assert.NilError(c, err)
	assert.Equal(c, len(containers), startCount+1)
	actual := fmt.Sprintf("%+v", containers[0])

	// empty Labels field triggered this bug, make sense to check for everything
	// cause even Ports for instance can trigger this bug
	// better safe than sorry..
	fields := []string{
		"ID",
		"Names",
		"Image",
		"Command",
		"Created",
		"Ports",
		"Labels",
		"Status",
		"NetworkSettings",
	}

	// decoding into types.Container do not work since it eventually unmarshal
	// and empty field to an empty go map, so we just check for a string
	for _, f := range fields {
		if !strings.Contains(actual, f) {
			c.Fatalf("Field %s is missing and it shouldn't", f)
		}
	}
}

func (s *DockerAPISuite) TestContainerAPIGetExport(c *testing.T) {
	// Not supported on Windows as Windows does not support docker export
	testRequires(c, DaemonIsLinux)
	const name = "exportcontainer"
	cli.DockerCmd(c, "run", "--name", name, "busybox", "touch", "/test")

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	body, err := apiClient.ContainerExport(testutil.GetContext(c), name)
	assert.NilError(c, err)
	defer body.Close()
	found := false
	for tarReader := tar.NewReader(body); ; {
		h, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if h != nil && h.Name == "test" {
			found = true
			break
		}
	}
	assert.Assert(c, found, "The created test file has not been found in the exported image")
}

func (s *DockerAPISuite) TestContainerAPIGetChanges(c *testing.T) {
	// Not supported on Windows as Windows does not support docker diff (/containers/name/changes)
	testRequires(c, DaemonIsLinux)
	const name = "changescontainer"
	cli.DockerCmd(c, "run", "--name", name, "busybox", "rm", "/etc/passwd")

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	changes, err := apiClient.ContainerDiff(testutil.GetContext(c), name)
	assert.NilError(c, err)

	// Check the changelog for removal of /etc/passwd
	success := false
	for _, elem := range changes {
		if elem.Path == "/etc/passwd" && elem.Kind == 2 {
			success = true
		}
	}
	assert.Assert(c, success, "/etc/passwd has been removed but is not present in the diff")
}

func (s *DockerAPISuite) TestGetContainerStats(c *testing.T) {
	const name = "statscontainer"
	runSleepingContainer(c, "--name", name)

	stream := make(chan containerstats.StreamItem)

	go func() {
		apiClient, err := client.NewClientWithOpts(client.FromEnv)
		assert.NilError(c, err)
		defer apiClient.Close()

		_, err = apiClient.ContainerStats(testutil.GetContext(c), name, containerstats.WithStream(stream))
		assert.NilError(c, err)
	}()

	// allow some time to stream the stats from the container
	time.Sleep(4 * time.Second)
	cli.DockerCmd(c, "rm", "-f", name)

	// collect the results from the stats stream or timeout and fail
	// if the stream was not disconnected.
	select {
	case <-time.After(2 * time.Second):
		c.Fatal("stream was not closed after container was removed")
	case <-stream:
	}
}

func (s *DockerAPISuite) TestGetContainerStatsRmRunning(c *testing.T) {
	id := runSleepingContainer(c)

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	stream := make(chan containerstats.StreamItem)
	_, err = apiClient.ContainerStats(testutil.GetContext(c), id, containerstats.WithStream(stream))
	assert.NilError(c, err)

	_, err = testutil.ChannelGetOne(c, stream, 2*time.Second)
	assert.NilError(c, err)

	// Now remove without `-f` and make sure we are still pulling stats
	_, _, err = dockerCmdWithError("rm", id)
	assert.Assert(c, err != nil, "rm should have failed but didn't")

	_, err = testutil.ChannelGetOne(c, stream, 2*time.Second)
	assert.NilError(c, err)

	cli.DockerCmd(c, "rm", "-f", id)

	select {
	case <-stream:
		c.Fatal("stream was not closed after container was removed")
	default:
	}
}

// ChannelBuffer holds a chan of byte array that can be populate in a goroutine.
type ChannelBuffer struct {
	C chan []byte
}

// Write implements Writer.
func (c *ChannelBuffer) Write(b []byte) (int, error) {
	c.C <- b
	return len(b), nil
}

// Close closes the go channel.
func (c *ChannelBuffer) Close() error {
	close(c.C)
	return nil
}

// ReadTimeout reads the content of the channel in the specified byte array with
// the specified duration as timeout.
func (c *ChannelBuffer) ReadTimeout(p []byte, n time.Duration) (int, error) {
	select {
	case b := <-c.C:
		return copy(p[0:], b), nil
	case <-time.After(n):
		return -1, errors.New("timeout reading from channel")
	}
}
