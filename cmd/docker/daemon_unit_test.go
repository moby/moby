// +build daemon

package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func stubRun(cmd *cobra.Command, args []string) error {
	return nil
}

func TestDaemonCommandHelp(t *testing.T) {
	cmd := newDaemonCommand()
	cmd.RunE = stubRun
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestDaemonCommand(t *testing.T) {
	cmd := newDaemonCommand()
	cmd.RunE = stubRun
	cmd.SetArgs([]string{"--containerd", "/foo"})
	err := cmd.Execute()
	assert.NoError(t, err)
}
