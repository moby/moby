package image

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/cli/internal/test"
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
		assert.Error(t, cmd.Execute(), expectedError)
	}
}

func TestCliNewTagCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewTagCommand(
		test.NewFakeCli(&fakeClient{
			imageTagFunc: func(image string, ref string) error {
				assert.Equal(t, image, "image1")
				assert.Equal(t, ref, "image2")
				return nil
			},
		}, buf))
	cmd.SetArgs([]string{"image1", "image2"})
	cmd.SetOutput(ioutil.Discard)
	assert.NoError(t, cmd.Execute())
	value, _ := cmd.Flags().GetBool("interspersed")
	assert.Equal(t, value, false)
}
