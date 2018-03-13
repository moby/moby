package main

import (
	"path/filepath"
	"testing"

	cliconfig "github.com/docker/docker/cli/config"
	"github.com/docker/docker/daemon/config"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/spf13/pflag"
)

func TestCommonOptionsInstallFlags(t *testing.T) {
	flags := pflag.NewFlagSet("testing", pflag.ContinueOnError)
	opts := newDaemonOptions(&config.Config{})
	opts.InstallFlags(flags)

	err := flags.Parse([]string{
		"--tlscacert=\"/foo/cafile\"",
		"--tlscert=\"/foo/cert\"",
		"--tlskey=\"/foo/key\"",
	})
	assert.Check(t, err)
	assert.Check(t, is.Equal("/foo/cafile", opts.TLSOptions.CAFile))
	assert.Check(t, is.Equal("/foo/cert", opts.TLSOptions.CertFile))
	assert.Check(t, is.Equal(opts.TLSOptions.KeyFile, "/foo/key"))
}

func defaultPath(filename string) string {
	return filepath.Join(cliconfig.Dir(), filename)
}

func TestCommonOptionsInstallFlagsWithDefaults(t *testing.T) {
	flags := pflag.NewFlagSet("testing", pflag.ContinueOnError)
	opts := newDaemonOptions(&config.Config{})
	opts.InstallFlags(flags)

	err := flags.Parse([]string{})
	assert.Check(t, err)
	assert.Check(t, is.Equal(defaultPath("ca.pem"), opts.TLSOptions.CAFile))
	assert.Check(t, is.Equal(defaultPath("cert.pem"), opts.TLSOptions.CertFile))
	assert.Check(t, is.Equal(defaultPath("key.pem"), opts.TLSOptions.KeyFile))
}
