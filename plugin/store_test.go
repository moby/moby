package plugin // import "github.com/docker/docker/plugin"

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/plugin/v2"
)

func TestFilterByCapNeg(t *testing.T) {
	p := v2.Plugin{PluginObj: types.Plugin{Name: "test:latest"}}
	iType := types.PluginInterfaceType{Capability: "volumedriver", Prefix: "docker", Version: "1.0"}
	i := types.PluginConfigInterface{Socket: "plugins.sock", Types: []types.PluginInterfaceType{iType}}
	p.PluginObj.Config.Interface = i

	_, err := p.FilterByCap("foobar")
	if err == nil {
		t.Fatalf("expected inadequate error, got %v", err)
	}
}

func TestFilterByCapPos(t *testing.T) {
	p := v2.Plugin{PluginObj: types.Plugin{Name: "test:latest"}}

	iType := types.PluginInterfaceType{Capability: "volumedriver", Prefix: "docker", Version: "1.0"}
	i := types.PluginConfigInterface{Socket: "plugins.sock", Types: []types.PluginInterfaceType{iType}}
	p.PluginObj.Config.Interface = i

	_, err := p.FilterByCap("volumedriver")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestStoreGetPluginNotMatchCapRefs(t *testing.T) {
	s := NewStore()
	p := v2.Plugin{PluginObj: types.Plugin{Name: "test:latest"}}

	iType := types.PluginInterfaceType{Capability: "whatever", Prefix: "docker", Version: "1.0"}
	i := types.PluginConfigInterface{Socket: "plugins.sock", Types: []types.PluginInterfaceType{iType}}
	p.PluginObj.Config.Interface = i

	if err := s.Add(&p); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Get("test", "volumedriver", plugingetter.Acquire); err == nil {
		t.Fatal("exepcted error when getting plugin that doesn't match the passed in capability")
	}

	if refs := p.GetRefCount(); refs != 0 {
		t.Fatalf("reference count should be 0, got: %d", refs)
	}

	p.PluginObj.Enabled = true
	if _, err := s.Get("test", "volumedriver", plugingetter.Acquire); err == nil {
		t.Fatal("exepcted error when getting plugin that doesn't match the passed in capability")
	}

	if refs := p.GetRefCount(); refs != 0 {
		t.Fatalf("reference count should be 0, got: %d", refs)
	}
}
