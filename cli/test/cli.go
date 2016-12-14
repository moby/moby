// Package test is a test-only package that can be used by other cli package to write unit test
package test

import (
	"io"

	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/client"
)

// FakeCli emulates the default DockerCli
type FakeCli struct {
	command.DockerCli
	client client.APIClient
	out    io.Writer
	in     io.ReadCloser
}

// NewFakeCli returns a Cli backed by the fakeCli
func NewFakeCli(client client.APIClient, out io.Writer, in io.ReadCloser) *FakeCli {
	return &FakeCli{
		client: client,
		out:    out,
		in:     in,
	}
}

// Client returns a docker API client
func (c *FakeCli) Client() client.APIClient {
	return c.client
}

// Out returns the output stream the cli should write on
func (c *FakeCli) Out() *command.OutStream {
	return command.NewOutStream(c.out)
}

// In returns thi input stream the cli will use
func (c *FakeCli) In() *command.InStream {
	return command.NewInStream(c.in)
}
