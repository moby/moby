package service

import (
	"io"

	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/service/progress"
	"github.com/docker/docker/pkg/jsonmessage"
	"golang.org/x/net/context"
)

// waitOnService waits for the service to converge. It outputs a progress bar,
// if appopriate based on the CLI flags.
func waitOnService(ctx context.Context, dockerCli *command.DockerCli, serviceID string, opts *serviceOptions) error {
	errChan := make(chan error, 1)
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		errChan <- progress.ServiceProgress(ctx, dockerCli.Client(), serviceID, pipeWriter)
	}()

	if opts.quiet {
		go func() {
			for {
				var buf [1024]byte
				if _, err := pipeReader.Read(buf[:]); err != nil {
					return
				}
			}
		}()
		return <-errChan
	}

	err := jsonmessage.DisplayJSONMessagesToStream(pipeReader, dockerCli.Out(), nil)
	if err == nil {
		err = <-errChan
	}
	return err
}
