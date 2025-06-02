package sockets

import (
	"net"
	"testing"
)

func createTestUnixSocket(t *testing.T, path string) (listener net.Listener) {
	l, err := NewUnixSocketWithOpts(path)
	if err != nil {
		t.Fatal(err)
	}
	return l
}
