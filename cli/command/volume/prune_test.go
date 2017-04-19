package volume

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil"
	"github.com/docker/docker/pkg/testutil/golden"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestVolumePruneErrors(t *testing.T) {
	testCases := []struct {
		args            []string
		flags           map[string]string
		volumePruneFunc func(args filters.Args) (types.VolumesPruneReport, error)
		expectedError   string
	}{
		{
			args:          []string{"foo"},
			expectedError: "accepts no argument",
		},
		{
			flags: map[string]string{
				"force": "true",
			},
			volumePruneFunc: func(args filters.Args) (types.VolumesPruneReport, error) {
				return types.VolumesPruneReport{}, errors.Errorf("error pruning volumes")
			},
			expectedError: "error pruning volumes",
		},
	}
	for _, tc := range testCases {
		cmd := NewPruneCommand(
			test.NewFakeCli(&fakeClient{
				volumePruneFunc: tc.volumePruneFunc,
			}, ioutil.Discard),
		)
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		cmd.SetOutput(ioutil.Discard)
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestVolumePruneForce(t *testing.T) {
	testCases := []struct {
		name            string
		volumePruneFunc func(args filters.Args) (types.VolumesPruneReport, error)
	}{
		{
			name: "empty",
		},
		{
			name:            "deletedVolumes",
			volumePruneFunc: simplePruneFunc,
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := NewPruneCommand(
			test.NewFakeCli(&fakeClient{
				volumePruneFunc: tc.volumePruneFunc,
			}, buf),
		)
		cmd.Flags().Set("force", "true")
		assert.NoError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), fmt.Sprintf("volume-prune.%s.golden", tc.name))
		testutil.EqualNormalizedString(t, testutil.RemoveSpace, actual, string(expected))
	}
}
func TestVolumePrunePromptYes(t *testing.T) {
	if runtime.GOOS == "windows" {
		// FIXME(vdemeester) make it work..
		t.Skip("skipping this test on Windows")
	}
	for _, input := range []string{"y", "Y"} {
		buf := new(bytes.Buffer)
		cli := test.NewFakeCli(&fakeClient{
			volumePruneFunc: simplePruneFunc,
		}, buf)

		cli.SetIn(ioutil.NopCloser(strings.NewReader(input)))
		cmd := NewPruneCommand(
			cli,
		)
		assert.NoError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), "volume-prune-yes.golden")
		testutil.EqualNormalizedString(t, testutil.RemoveSpace, actual, string(expected))
	}
}

func TestVolumePrunePromptNo(t *testing.T) {
	if runtime.GOOS == "windows" {
		// FIXME(vdemeester) make it work..
		t.Skip("skipping this test on Windows")
	}
	for _, input := range []string{"n", "N", "no", "anything", "really"} {
		buf := new(bytes.Buffer)
		cli := test.NewFakeCli(&fakeClient{
			volumePruneFunc: simplePruneFunc,
		}, buf)

		cli.SetIn(ioutil.NopCloser(strings.NewReader(input)))
		cmd := NewPruneCommand(
			cli,
		)
		assert.NoError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), "volume-prune-no.golden")
		testutil.EqualNormalizedString(t, testutil.RemoveSpace, actual, string(expected))
	}
}

func simplePruneFunc(args filters.Args) (types.VolumesPruneReport, error) {
	return types.VolumesPruneReport{
		VolumesDeleted: []string{
			"foo", "bar", "baz",
		},
		SpaceReclaimed: 2000,
	}, nil
}
