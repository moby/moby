package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	containerpkg "github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
)

type containerManager struct {
	tmpContainers map[string]struct{}
	backend       builder.ExecBackend
}

// newContainerManager creates a new container backend
func newContainerManager(docker builder.ExecBackend) *containerManager {
	return &containerManager{
		backend:       docker,
		tmpContainers: make(map[string]struct{}),
	}
}

// Create a container
func (c *containerManager) Create(ctx context.Context, runConfig *container.Config, hostConfig *container.HostConfig) (container.CreateResponse, error) {
	ctr, err := c.backend.ContainerCreateIgnoreImagesArgsEscaped(ctx, backend.ContainerCreateConfig{
		Config:     runConfig,
		HostConfig: hostConfig,
	})
	if err != nil {
		return ctr, err
	}
	c.tmpContainers[ctr.ID] = struct{}{}
	return ctr, nil
}

var errCancelled = errors.New("build cancelled")

// Run a container by ID
func (c *containerManager) Run(ctx context.Context, cID string, stdout, stderr io.Writer) (err error) {
	attached := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.backend.ContainerAttachRaw(cID, nil, stdout, stderr, true, attached)
	}()
	select {
	case err := <-errCh:
		return err
	case <-attached:
	}

	finished := make(chan struct{})
	cancelErrCh := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			log.G(ctx).Debugln("Build cancelled, removing container:", cID)
			err = c.backend.ContainerRm(cID, &backend.ContainerRmConfig{ForceRemove: true, RemoveVolume: true})
			if err != nil {
				_, _ = fmt.Fprintf(stdout, "Removing container %s: %v\n", stringid.TruncateID(cID), err)
			}
			cancelErrCh <- errCancelled
		case <-finished:
			cancelErrCh <- nil
		}
	}()

	if err := c.backend.ContainerStart(ctx, cID, "", ""); err != nil {
		close(finished)
		logCancellationError(cancelErrCh, "error from ContainerStart: "+err.Error())
		return err
	}

	// Block on reading output from container, stop on err or chan closed
	if err := <-errCh; err != nil {
		close(finished)
		logCancellationError(cancelErrCh, "error from errCh: "+err.Error())
		return err
	}

	waitC, err := c.backend.ContainerWait(ctx, cID, containerpkg.WaitConditionNotRunning)
	if err != nil {
		close(finished)
		logCancellationError(cancelErrCh, fmt.Sprintf("unable to begin ContainerWait: %s", err))
		return err
	}

	if status := <-waitC; status.ExitCode() != 0 {
		close(finished)
		logCancellationError(cancelErrCh,
			fmt.Sprintf("a non-zero code from ContainerWait: %d", status.ExitCode()))
		return &statusCodeError{code: status.ExitCode(), err: status.Err()}
	}

	close(finished)
	return <-cancelErrCh
}

func logCancellationError(cancelErrCh chan error, msg string) {
	if cancelErr := <-cancelErrCh; cancelErr != nil {
		log.G(context.TODO()).Debugf("Build cancelled (%v): %s", cancelErr, msg)
	}
}

type statusCodeError struct {
	code int
	err  error
}

func (e *statusCodeError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *statusCodeError) StatusCode() int {
	return e.code
}

// RemoveAll containers managed by this container manager
func (c *containerManager) RemoveAll(stdout io.Writer) {
	for containerID := range c.tmpContainers {
		if err := c.backend.ContainerRm(containerID, &backend.ContainerRmConfig{ForceRemove: true, RemoveVolume: true}); err != nil && !errdefs.IsNotFound(err) {
			_, _ = fmt.Fprintf(stdout, "Removing intermediate container %s: %v\n", stringid.TruncateID(containerID), err)
			continue
		}
		delete(c.tmpContainers, containerID)
		_, _ = fmt.Fprintf(stdout, " ---> Removed intermediate container %s\n", stringid.TruncateID(containerID))
	}
}
