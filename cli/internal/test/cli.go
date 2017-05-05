package test

import (
	"io"
	"io/ioutil"
	"strings"

	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/config/configfile"
	"github.com/docker/docker/cli/config/credentials"
	"github.com/docker/docker/client"
)

// FakeCli emulates the default DockerCli
type FakeCli struct {
	command.DockerCli
	client     client.APIClient
	configfile *configfile.ConfigFile
	out        *command.OutStream
	err        io.Writer
	in         *command.InStream
	store      credentials.Store
}

// NewFakeCli returns a Cli backed by the fakeCli
func NewFakeCli(client client.APIClient, out io.Writer) *FakeCli {
	return &FakeCli{
		client: client,
		out:    command.NewOutStream(out),
		err:    ioutil.Discard,
		in:     command.NewInStream(ioutil.NopCloser(strings.NewReader(""))),
	}
}

// SetIn sets the input of the cli to the specified ReadCloser
func (c *FakeCli) SetIn(in *command.InStream) {
	c.in = in
}

// SetErr sets the stderr stream for the cli to the specified io.Writer
func (c *FakeCli) SetErr(err io.Writer) {
	c.err = err
}

// SetConfigfile sets the "fake" config file
func (c *FakeCli) SetConfigfile(configfile *configfile.ConfigFile) {
	c.configfile = configfile
}

// Client returns a docker API client
func (c *FakeCli) Client() client.APIClient {
	return c.client
}

// Out returns the output stream (stdout) the cli should write on
func (c *FakeCli) Out() *command.OutStream {
	return c.out
}

// Err returns the output stream (stderr) the cli should write on
func (c *FakeCli) Err() io.Writer {
	return c.err
}

// In returns the input stream the cli will use
func (c *FakeCli) In() *command.InStream {
	return c.in
}

// ConfigFile returns the cli configfile object (to get client configuration)
func (c *FakeCli) ConfigFile() *configfile.ConfigFile {
	return c.configfile
}

// CredentialsStore returns the fake store the cli will use
func (c *FakeCli) CredentialsStore(serverAddress string) credentials.Store {
	if c.store == nil {
		c.store = NewFakeStore()
	}
	return c.store
}
