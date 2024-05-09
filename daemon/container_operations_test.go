package daemon

import (
	"encoding/json"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/container"
	"github.com/docker/docker/libnetwork"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDNSNamesOrder(t *testing.T) {
	d := &Daemon{}
	ctr := &container.Container{
		ID:   "35de8003b19e27f636fc6ecbf4d7072558b872a8544f287fd69ad8182ad59023",
		Name: "foobar",
		Config: &containertypes.Config{
			Hostname: "baz",
		},
		HostConfig: &containertypes.HostConfig{},
	}
	nw := buildNetwork(t, map[string]any{
		"id":          "1234567890",
		"name":        "testnet",
		"networkType": "bridge",
		"enableIPv6":  false,
	})
	epSettings := &networktypes.EndpointSettings{
		Aliases: []string{"myctr"},
	}

	if err := d.updateNetworkConfig(ctr, nw, epSettings); err != nil {
		t.Fatal(err)
	}

	assert.Check(t, is.DeepEqual(epSettings.DNSNames, []string{"foobar", "myctr", "35de8003b19e", "baz"}))
}

func buildNetwork(t *testing.T, config map[string]any) *libnetwork.Network {
	t.Helper()

	b, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}

	nw := &libnetwork.Network{}
	if err := nw.UnmarshalJSON(b); err != nil {
		t.Fatal(err)
	}

	return nw
}
