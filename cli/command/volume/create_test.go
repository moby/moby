package volume

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestVolumeCreateErrors(t *testing.T) {
	testCases := []struct {
		args             []string
		flags            map[string]string
		volumeCreateFunc func(volumetypes.VolumesCreateBody) (types.Volume, error)
		expectedError    string
	}{
		{
			args: []string{"volumeName"},
			flags: map[string]string{
				"name": "volumeName",
			},
			expectedError: "Conflicting options: either specify --name or provide positional arg, not both",
		},
		{
			args:          []string{"too", "many"},
			expectedError: "requires at most 1 argument(s)",
		},
		{
			volumeCreateFunc: func(createBody volumetypes.VolumesCreateBody) (types.Volume, error) {
				return types.Volume{}, errors.Errorf("error creating volume")
			},
			expectedError: "error creating volume",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newCreateCommand(
			test.NewFakeCli(&fakeClient{
				volumeCreateFunc: tc.volumeCreateFunc,
			}, buf),
		)
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		cmd.SetOutput(ioutil.Discard)
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestVolumeCreateWithName(t *testing.T) {
	name := "foo"
	buf := new(bytes.Buffer)
	cli := test.NewFakeCli(&fakeClient{
		volumeCreateFunc: func(body volumetypes.VolumesCreateBody) (types.Volume, error) {
			if body.Name != name {
				return types.Volume{}, errors.Errorf("expected name %q, got %q", name, body.Name)
			}
			return types.Volume{
				Name: body.Name,
			}, nil
		},
	}, buf)

	// Test by flags
	cmd := newCreateCommand(cli)
	cmd.Flags().Set("name", name)
	assert.NoError(t, cmd.Execute())
	assert.Equal(t, name, strings.TrimSpace(buf.String()))

	// Then by args
	buf.Reset()
	cmd = newCreateCommand(cli)
	cmd.SetArgs([]string{name})
	assert.NoError(t, cmd.Execute())
	assert.Equal(t, name, strings.TrimSpace(buf.String()))
}

func TestVolumeCreateWithFlags(t *testing.T) {
	expectedDriver := "foo"
	expectedOpts := map[string]string{
		"bar": "1",
		"baz": "baz",
	}
	expectedLabels := map[string]string{
		"lbl1": "v1",
		"lbl2": "v2",
	}
	name := "banana"

	buf := new(bytes.Buffer)
	cli := test.NewFakeCli(&fakeClient{
		volumeCreateFunc: func(body volumetypes.VolumesCreateBody) (types.Volume, error) {
			if body.Name != "" {
				return types.Volume{}, errors.Errorf("expected empty name, got %q", body.Name)
			}
			if body.Driver != expectedDriver {
				return types.Volume{}, errors.Errorf("expected driver %q, got %q", expectedDriver, body.Driver)
			}
			if !compareMap(body.DriverOpts, expectedOpts) {
				return types.Volume{}, errors.Errorf("expected drivers opts %v, got %v", expectedOpts, body.DriverOpts)
			}
			if !compareMap(body.Labels, expectedLabels) {
				return types.Volume{}, errors.Errorf("expected labels %v, got %v", expectedLabels, body.Labels)
			}
			return types.Volume{
				Name: name,
			}, nil
		},
	}, buf)

	cmd := newCreateCommand(cli)
	cmd.Flags().Set("driver", "foo")
	cmd.Flags().Set("opt", "bar=1")
	cmd.Flags().Set("opt", "baz=baz")
	cmd.Flags().Set("label", "lbl1=v1")
	cmd.Flags().Set("label", "lbl2=v2")
	assert.NoError(t, cmd.Execute())
	assert.Equal(t, name, strings.TrimSpace(buf.String()))
}

func compareMap(actual map[string]string, expected map[string]string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for key, value := range actual {
		if expectedValue, ok := expected[key]; ok {
			if expectedValue != value {
				return false
			}
		} else {
			return false
		}
	}
	return true
}
