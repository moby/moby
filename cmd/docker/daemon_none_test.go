// +build !daemon

package main

import (
	"strings"
	"testing"
)

func TestCmdDaemon(t *testing.T) {
	proxy := NewDaemonProxy()
	err := proxy.CmdDaemon("--help")
	if err == nil {
		t.Fatal("Expected CmdDaemon to fail on Windows.")
	}

	if !strings.Contains(err.Error(), "Please run `dockerd`") {
		t.Fatalf("Expected an error about running dockerd, got %s", err)
	}
}
