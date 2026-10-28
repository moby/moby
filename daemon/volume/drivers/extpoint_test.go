package drivers

import (
	"strings"
	"testing"

	volumetestutils "github.com/moby/moby/v2/daemon/volume/testutils"
	"github.com/moby/moby/v2/pkg/plugins"
)

func TestGetDriver(t *testing.T) {
	s := NewStore(nil)
	_, err := s.GetDriver("missing")
	if err == nil {
		t.Fatal("Expected error, was nil")
	}
	s.Register(volumetestutils.NewFakeDriver("fake"), "fake")

	d, err := s.GetDriver("fake")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name() != "fake" {
		t.Fatalf("Expected fake driver, got %s\n", d.Name())
	}
}

type dummyNilV1Plugin struct {
	name string
}

func (d *dummyNilV1Plugin) Name() string {
	return d.name
}

func (d *dummyNilV1Plugin) ScopedPath(s string) string {
	return s
}

func (d *dummyNilV1Plugin) IsV1() bool {
	return true
}

func (d *dummyNilV1Plugin) Client() *plugins.Client {
	return nil
}

func TestMakePluginAdapterNilClient(t *testing.T) {
	pluginName := "nil-plugin"
	p := &dummyNilV1Plugin{name: pluginName}

	adapter, err := makePluginAdapter(p)
	if adapter != nil {
		t.Fatalf("Expected adapter to be nil, got %#v", adapter)
	}
	if err == nil {
		t.Fatal("Expected error, was nil")
	}
	if !strings.Contains(err.Error(), "client is nil") {
		t.Fatalf("Expected error mentioning nil client, got: %v", err)
	}
	if !strings.Contains(err.Error(), pluginName) {
		t.Fatalf("Expected error to contain plugin name %q, got: %v", pluginName, err)
	}
}
