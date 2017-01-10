package convert

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	composetypes "github.com/docker/docker/cli/compose/types"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestNamespaceScope(t *testing.T) {
	scoped := Namespace{name: "foo"}.Scope("bar")
	assert.Equal(t, scoped, "foo_bar")
}

func TestAddStackLabel(t *testing.T) {
	labels := map[string]string{
		"something": "labeled",
	}
	actual := AddStackLabel(Namespace{name: "foo"}, labels)
	expected := map[string]string{
		"something":    "labeled",
		LabelNamespace: "foo",
	}
	assert.DeepEqual(t, actual, expected)
}

func TestNetworks(t *testing.T) {
	namespace := Namespace{name: "foo"}
	serviceNetworks := map[string]struct{}{
		"normal":  {},
		"outside": {},
		"default": {},
	}
	source := networkMap{
		"normal": composetypes.NetworkConfig{
			Driver: "overlay",
			DriverOpts: map[string]string{
				"opt": "value",
			},
			Ipam: composetypes.IPAMConfig{
				Driver: "driver",
				Config: []*composetypes.IPAMPool{
					{
						Subnet: "10.0.0.0",
					},
				},
			},
			Labels: map[string]string{
				"something": "labeled",
			},
		},
		"outside": composetypes.NetworkConfig{
			External: composetypes.External{
				External: true,
				Name:     "special",
			},
		},
	}
	expected := map[string]types.NetworkCreate{
		"default": {
			Labels: map[string]string{
				LabelNamespace: "foo",
			},
		},
		"normal": {
			Driver: "overlay",
			IPAM: &network.IPAM{
				Driver: "driver",
				Config: []network.IPAMConfig{
					{
						Subnet: "10.0.0.0",
					},
				},
			},
			Options: map[string]string{
				"opt": "value",
			},
			Labels: map[string]string{
				LabelNamespace: "foo",
				"something":    "labeled",
			},
		},
	}

	networks, externals := Networks(namespace, source, serviceNetworks)
	assert.DeepEqual(t, networks, expected)
	assert.DeepEqual(t, externals, []string{"special"})
}
