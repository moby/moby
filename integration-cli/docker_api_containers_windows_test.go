//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"testing"

	winio "github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func (s *DockerSuite) TestContainersAPICreateMountsBindNamedPipe(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsWindowsAtLeastBuild(osversion.RS3)) // Named pipe support was added in RS3

	// Create a host pipe to map into the container
	hostPipeName := fmt.Sprintf(`\\.\pipe\docker-cli-test-pipe-%x`, rand.Uint64())
	pc := &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;AU)", // Allow all users access to the pipe
	}
	l, err := winio.ListenPipe(hostPipeName, pc)
	if err != nil {
		c.Fatal(err)
	}
	defer l.Close()

	// Asynchronously read data that the container writes to the mapped pipe.
	var b []byte
	ch := make(chan error)
	go func() {
		conn, err := l.Accept()
		if err == nil {
			b, err = io.ReadAll(conn)
			conn.Close()
		}
		ch <- err
	}()

	containerPipeName := `\\.\pipe\docker-cli-test-pipe`
	text := "hello from a pipe"
	cmd := fmt.Sprintf("echo %s > %s", text, containerPipeName)
	name := "test-bind-npipe"

	ctx := context.Background()
	client := testEnv.APIClient()
	_, err = client.ContainerCreate(ctx,
		&container.Config{
			Image: testEnv.PlatformDefaults.BaseImage,
			Cmd:   []string{"cmd", "/c", cmd},
		}, &container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:   "npipe",
					Source: hostPipeName,
					Target: containerPipeName,
				},
			},
		},
		nil, nil, name)
	assert.NilError(c, err)

	err = client.ContainerStart(ctx, name, types.ContainerStartOptions{})
	assert.NilError(c, err)

	err = <-ch
	assert.NilError(c, err)
	assert.Check(c, is.Equal(text, strings.TrimSpace(string(b))))
}

func mountWrapper(device, target, mType, options string) error {
	// This should never be called.
	return errors.Errorf("there is no implementation of Mount on this platform")
}
