package store

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/plugin/v2"
)

func TestFilterByCapNeg(t *testing.T) {
	p := v2.NewPlugin("test", "1234567890", "/run/docker", "/var/lib/docker/plugins", "latest")

	iType := types.PluginInterfaceType{"volumedriver", "docker", "1.0"}
	i := types.PluginManifestInterface{"plugins.sock", []types.PluginInterfaceType{iType}}
	p.PluginObj.Manifest.Interface = i

	_, err := p.FilterByCap("foobar")
	if err == nil {
		t.Fatalf("expected inadequate error, got %v", err)
	}
}

func TestFilterByCapPos(t *testing.T) {
	p := v2.NewPlugin("test", "1234567890", "/run/docker", "/var/lib/docker/plugins", "latest")

	iType := types.PluginInterfaceType{"volumedriver", "docker", "1.0"}
	i := types.PluginManifestInterface{"plugins.sock", []types.PluginInterfaceType{iType}}
	p.PluginObj.Manifest.Interface = i

	_, err := p.FilterByCap("volumedriver")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
