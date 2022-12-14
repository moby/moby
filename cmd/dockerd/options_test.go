package main

import (
	"path/filepath"
	"testing"

	"github.com/docker/docker/daemon/config"
	"github.com/spf13/pflag"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCommonOptionsInstallFlags(t *testing.T) {
	flags := pflag.NewFlagSet("testing", pflag.ContinueOnError)
	opts := newDaemonOptions(&config.Config{})
	opts.installFlags(flags)

	err := flags.Parse([]string{
		"--tlscacert=/foo/cafile",
		"--tlscert=/foo/cert",
		"--tlskey=/foo/key",
	})
	assert.Check(t, err)
	assert.Check(t, is.Equal("/foo/cafile", opts.TLSOptions.CAFile))
	assert.Check(t, is.Equal("/foo/cert", opts.TLSOptions.CertFile))
	assert.Check(t, is.Equal(opts.TLSOptions.KeyFile, "/foo/key"))
}

func TestCommonOptionsInstallFlagsWithDefaults(t *testing.T) {
	flags := pflag.NewFlagSet("testing", pflag.ContinueOnError)
	opts := newDaemonOptions(&config.Config{})
	opts.installFlags(flags)

	err := flags.Parse([]string{})
	assert.Check(t, err)
	assert.Check(t, is.Equal(filepath.Join(defaultCertPath(), "ca.pem"), opts.TLSOptions.CAFile))
	assert.Check(t, is.Equal(filepath.Join(defaultCertPath(), "cert.pem"), opts.TLSOptions.CertFile))
	assert.Check(t, is.Equal(filepath.Join(defaultCertPath(), "key.pem"), opts.TLSOptions.KeyFile))
}
