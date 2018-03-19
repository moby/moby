// +build linux freebsd

package main

import (
	"testing"

	"github.com/docker/docker/daemon/config"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/spf13/pflag"
)

func TestDaemonParseShmSize(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	conf := &config.Config{}
	installConfigFlags(conf, flags)
	// By default `--default-shm-size=64M`
	assert.Check(t, is.Equal(int64(64*1024*1024), conf.ShmSize.Value()))
	assert.Check(t, flags.Set("default-shm-size", "128M"))
	assert.Check(t, is.Equal(int64(128*1024*1024), conf.ShmSize.Value()))
}
