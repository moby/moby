package volumedrivers

import (
	"testing"

	pluginstore "github.com/docker/docker/plugin/store"
	volumetestutils "github.com/docker/docker/volume/testutils"
)

func TestGetDriver(t *testing.T) {
	pluginStore := pluginstore.NewStore("/var/lib/docker")
	RegisterPluginGetter(pluginStore)

	_, err := GetDriver("missing")
	if err == nil {
		t.Fatal("Expected error, was nil")
	}
	Register(volumetestutils.NewFakeDriver("fake"), "fake")

	d, err := GetDriver("fake")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name() != "fake" {
		t.Fatalf("Expected fake driver, got %s\n", d.Name())
	}
}
