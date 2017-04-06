// +build daemon

package main

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/spf13/cobra"
)

func stubRun(cmd *cobra.Command, args []string) error {
	return nil
}

func TestDaemonCommandHelp(t *testing.T) {
	cmd := newDaemonCommand()
	cmd.RunE = stubRun
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	assert.NilError(t, err)
}

func TestDaemonCommand(t *testing.T) {
	cmd := newDaemonCommand()
	cmd.RunE = stubRun
	cmd.SetArgs([]string{"--containerd", "/foo"})
	err := cmd.Execute()
	assert.NilError(t, err)
}
