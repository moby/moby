package volume

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli/internal/test"
	"github.com/pkg/errors"
	// Import builders to get the builder function as package function
	. "github.com/docker/docker/cli/internal/test/builders"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/docker/pkg/testutil/golden"
)

func TestVolumeInspectErrors(t *testing.T) {
	testCases := []struct {
		args              []string
		flags             map[string]string
		volumeInspectFunc func(volumeID string) (types.Volume, error)
		expectedError     string
	}{
		{
			expectedError: "requires at least 1 argument",
		},
		{
			args: []string{"foo"},
			volumeInspectFunc: func(volumeID string) (types.Volume, error) {
				return types.Volume{}, errors.Errorf("error while inspecting the volume")
			},
			expectedError: "error while inspecting the volume",
		},
		{
			args: []string{"foo"},
			flags: map[string]string{
				"format": "{{invalid format}}",
			},
			expectedError: "Template parsing error",
		},
		{
			args: []string{"foo", "bar"},
			volumeInspectFunc: func(volumeID string) (types.Volume, error) {
				if volumeID == "foo" {
					return types.Volume{
						Name: "foo",
					}, nil
				}
				return types.Volume{}, errors.Errorf("error while inspecting the volume")
			},
			expectedError: "error while inspecting the volume",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newInspectCommand(
			test.NewFakeCli(&fakeClient{
				volumeInspectFunc: tc.volumeInspectFunc,
			}, buf),
		)
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		cmd.SetOutput(ioutil.Discard)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestVolumeInspectWithoutFormat(t *testing.T) {
	testCases := []struct {
		name              string
		args              []string
		volumeInspectFunc func(volumeID string) (types.Volume, error)
	}{
		{
			name: "single-volume",
			args: []string{"foo"},
			volumeInspectFunc: func(volumeID string) (types.Volume, error) {
				if volumeID != "foo" {
					return types.Volume{}, errors.Errorf("Invalid volumeID, expected %s, got %s", "foo", volumeID)
				}
				return *Volume(), nil
			},
		},
		{
			name: "multiple-volume-with-labels",
			args: []string{"foo", "bar"},
			volumeInspectFunc: func(volumeID string) (types.Volume, error) {
				return *Volume(VolumeName(volumeID), VolumeLabels(map[string]string{
					"foo": "bar",
				})), nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newInspectCommand(
			test.NewFakeCli(&fakeClient{
				volumeInspectFunc: tc.volumeInspectFunc,
			}, buf),
		)
		cmd.SetArgs(tc.args)
		assert.NilError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), fmt.Sprintf("volume-inspect-without-format.%s.golden", tc.name))
		assert.EqualNormalizedString(t, assert.RemoveSpace, actual, string(expected))
	}
}

func TestVolumeInspectWithFormat(t *testing.T) {
	volumeInspectFunc := func(volumeID string) (types.Volume, error) {
		return *Volume(VolumeLabels(map[string]string{
			"foo": "bar",
		})), nil
	}
	testCases := []struct {
		name              string
		format            string
		args              []string
		volumeInspectFunc func(volumeID string) (types.Volume, error)
	}{
		{
			name:              "simple-template",
			format:            "{{.Name}}",
			args:              []string{"foo"},
			volumeInspectFunc: volumeInspectFunc,
		},
		{
			name:              "json-template",
			format:            "{{json .Labels}}",
			args:              []string{"foo"},
			volumeInspectFunc: volumeInspectFunc,
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newInspectCommand(
			test.NewFakeCli(&fakeClient{
				volumeInspectFunc: tc.volumeInspectFunc,
			}, buf),
		)
		cmd.SetArgs(tc.args)
		cmd.Flags().Set("format", tc.format)
		assert.NilError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), fmt.Sprintf("volume-inspect-with-format.%s.golden", tc.name))
		assert.EqualNormalizedString(t, assert.RemoveSpace, actual, string(expected))
	}
}
