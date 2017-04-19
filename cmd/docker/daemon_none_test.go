// +build !daemon

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDaemonCommand(t *testing.T) {
	cmd := newDaemonCommand()
	cmd.SetArgs([]string{"--version"})
	err := cmd.Execute()

	assert.EqualError(t, err, "Please run `dockerd`")
}
