package image

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil"
	"github.com/docker/docker/pkg/testutil/golden"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewRemoveCommandAlias(t *testing.T) {
	cmd := newRemoveCommand(test.NewFakeCli(&fakeClient{}, new(bytes.Buffer)))
	assert.True(t, cmd.HasAlias("rmi"))
	assert.True(t, cmd.HasAlias("remove"))
	assert.False(t, cmd.HasAlias("other"))
}

func TestNewRemoveCommandErrors(t *testing.T) {
	testCases := []struct {
		name            string
		args            []string
		expectedError   string
		imageRemoveFunc func(image string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error)
	}{
		{
			name:          "wrong args",
			expectedError: "requires at least 1 argument(s).",
		},
		{
			name:          "ImageRemove fail",
			args:          []string{"arg1"},
			expectedError: "error removing image",
			imageRemoveFunc: func(image string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
				assert.False(t, options.Force)
				assert.True(t, options.PruneChildren)
				return []types.ImageDeleteResponseItem{}, errors.Errorf("error removing image")
			},
		},
	}
	for _, tc := range testCases {
		cmd := NewRemoveCommand(test.NewFakeCli(&fakeClient{
			imageRemoveFunc: tc.imageRemoveFunc,
		}, new(bytes.Buffer)))
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNewRemoveCommandSuccess(t *testing.T) {
	testCases := []struct {
		name            string
		args            []string
		imageRemoveFunc func(image string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error)
	}{
		{
			name: "Image Deleted",
			args: []string{"image1"},
			imageRemoveFunc: func(image string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
				assert.Equal(t, "image1", image)
				return []types.ImageDeleteResponseItem{{Deleted: image}}, nil
			},
		},
		{
			name: "Image Untagged",
			args: []string{"image1"},
			imageRemoveFunc: func(image string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
				assert.Equal(t, "image1", image)
				return []types.ImageDeleteResponseItem{{Untagged: image}}, nil
			},
		},
		{
			name: "Image Deleted and Untagged",
			args: []string{"image1", "image2"},
			imageRemoveFunc: func(image string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
				if image == "image1" {
					return []types.ImageDeleteResponseItem{{Untagged: image}}, nil
				}
				return []types.ImageDeleteResponseItem{{Deleted: image}}, nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := NewRemoveCommand(test.NewFakeCli(&fakeClient{
			imageRemoveFunc: tc.imageRemoveFunc,
		}, buf))
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		assert.NoError(t, cmd.Execute())
		err := cmd.Execute()
		assert.NoError(t, err)
		actual := buf.String()
		expected := string(golden.Get(t, []byte(actual), fmt.Sprintf("remove-command-success.%s.golden", tc.name))[:])
		testutil.EqualNormalizedString(t, testutil.RemoveSpace, actual, expected)
	}
}
