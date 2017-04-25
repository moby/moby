package image

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli/config/configfile"
	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil"
	"github.com/docker/docker/pkg/testutil/golden"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewImagesCommandErrors(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		expectedError string
		imageListFunc func(options types.ImageListOptions) ([]types.ImageSummary, error)
	}{
		{
			name:          "wrong-args",
			args:          []string{"arg1", "arg2"},
			expectedError: "requires at most 1 argument(s).",
		},
		{
			name:          "failed-list",
			expectedError: "something went wrong",
			imageListFunc: func(options types.ImageListOptions) ([]types.ImageSummary, error) {
				return []types.ImageSummary{{}}, errors.Errorf("something went wrong")
			},
		},
	}
	for _, tc := range testCases {
		cmd := NewImagesCommand(test.NewFakeCli(&fakeClient{imageListFunc: tc.imageListFunc}, new(bytes.Buffer)))
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNewImagesCommandSuccess(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		imageFormat   string
		imageListFunc func(options types.ImageListOptions) ([]types.ImageSummary, error)
	}{
		{
			name: "simple",
		},
		{
			name:        "format",
			imageFormat: "raw",
		},
		{
			name:        "quiet-format",
			args:        []string{"-q"},
			imageFormat: "table",
		},
		{
			name: "match-name",
			args: []string{"image"},
			imageListFunc: func(options types.ImageListOptions) ([]types.ImageSummary, error) {
				assert.Equal(t, "image", options.Filters.Get("reference")[0])
				return []types.ImageSummary{{}}, nil
			},
		},
		{
			name: "filters",
			args: []string{"--filter", "name=value"},
			imageListFunc: func(options types.ImageListOptions) ([]types.ImageSummary, error) {
				assert.Equal(t, "value", options.Filters.Get("name")[0])
				return []types.ImageSummary{{}}, nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cli := test.NewFakeCli(&fakeClient{imageListFunc: tc.imageListFunc}, buf)
		cli.SetConfigfile(&configfile.ConfigFile{ImagesFormat: tc.imageFormat})
		cmd := NewImagesCommand(cli)
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		err := cmd.Execute()
		assert.NoError(t, err)
		actual := buf.String()
		expected := string(golden.Get(t, []byte(actual), fmt.Sprintf("list-command-success.%s.golden", tc.name))[:])
		testutil.EqualNormalizedString(t, testutil.RemoveSpace, actual, expected)
	}
}

func TestNewListCommandAlias(t *testing.T) {
	cmd := newListCommand(test.NewFakeCli(&fakeClient{}, new(bytes.Buffer)))
	assert.True(t, cmd.HasAlias("images"))
	assert.True(t, cmd.HasAlias("list"))
	assert.False(t, cmd.HasAlias("other"))
}
