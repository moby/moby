// +build !daemon

package main

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
)

func TestDaemonCommand(t *testing.T) {
	cmd := newDaemonCommand()
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()

	assert.Error(t, err, "Please run `dockerd`")
}
