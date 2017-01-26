package convert

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	composetypes "github.com/docker/docker/cli/compose/types"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/docker/pkg/testutil/tempfile"
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

func TestSecrets(t *testing.T) {
	namespace := Namespace{name: "foo"}

	secretText := "this is the first secret"
	secretFile := tempfile.NewTempFile(t, "convert-secrets", secretText)
	defer secretFile.Remove()

	source := map[string]composetypes.SecretConfig{
		"one": {
			File:   secretFile.Name(),
			Labels: map[string]string{"monster": "mash"},
		},
		"ext": {
			External: composetypes.External{
				External: true,
			},
		},
	}

	specs, err := Secrets(namespace, source)
	assert.NilError(t, err)
	assert.Equal(t, len(specs), 1)
	secret := specs[0]
	assert.Equal(t, secret.Name, "foo_one")
	assert.DeepEqual(t, secret.Labels, map[string]string{
		"monster":      "mash",
		LabelNamespace: "foo",
	})
	assert.DeepEqual(t, secret.Data, []byte(secretText))
}
