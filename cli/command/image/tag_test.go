package image

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestCliNewTagCommandErrors(t *testing.T) {
	testCases := [][]string{
		{},
		{"image1"},
		{"image1", "image2", "image3"},
	}
	expectedError := "\"tag\" requires exactly 2 argument(s)."
	buf := new(bytes.Buffer)
	for _, args := range testCases {
		cmd := NewTagCommand(test.NewFakeCli(&fakeClient{}, buf))
		cmd.SetArgs(args)
		cmd.SetOutput(ioutil.Discard)
		testutil.ErrorContains(t, cmd.Execute(), expectedError)
	}
}

func TestCliNewTagCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewTagCommand(
		test.NewFakeCli(&fakeClient{
			imageTagFunc: func(image string, ref string) error {
				assert.Equal(t, "image1", image)
				assert.Equal(t, "image2", ref)
				return nil
			},
		}, buf))
	cmd.SetArgs([]string{"image1", "image2"})
	cmd.SetOutput(ioutil.Discard)
	assert.NoError(t, cmd.Execute())
	value, _ := cmd.Flags().GetBool("interspersed")
	assert.False(t, value)
}
