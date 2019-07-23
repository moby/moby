package plugin // import "github.com/docker/docker/daemon/plugin"

import (
	"fmt"
	"testing"
)

func init() {
	Register(&Registration{
		Type: GRPCPlugin,
		ID:   "testplugin",
		InitFn: func(ic *Context) (interface{}, error) {
			return struct{}{}, nil
		},
	})
}

func TestDaemonPluginInit(t *testing.T) {
	plugins := Graph()
	registered := false
	for _, p := range plugins {
		fmt.Println(p.URI())
		if p.URI() == "io.docker.grpc.v1.testplugin" {
			registered = true
		}
	}

	if !registered {
		t.Fatal("expected test plugin to be registered")
	}
}
