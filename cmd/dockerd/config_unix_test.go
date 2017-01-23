// +build linux,!solaris freebsd,!solaris

package main

import (
	"runtime"
	"testing"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/spf13/pflag"
)

func TestDaemonParseShmSize(t *testing.T) {
	if runtime.GOOS == "solaris" {
		t.Skip("ShmSize not supported on Solaris\n")
	}
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	conf := &config.Config{}
	installConfigFlags(conf, flags)
	// By default `--default-shm-size=64M`
	expectedValue := 64 * 1024 * 1024
	if conf.ShmSize.Value() != int64(expectedValue) {
		t.Fatalf("expected default shm size %d, got %d", expectedValue, conf.ShmSize.Value())
	}
	assert.NilError(t, flags.Set("default-shm-size", "128M"))
	expectedValue = 128 * 1024 * 1024
	if conf.ShmSize.Value() != int64(expectedValue) {
		t.Fatalf("expected default shm size %d, got %d", expectedValue, conf.ShmSize.Value())
	}
}
