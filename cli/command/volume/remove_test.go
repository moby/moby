package volume

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/pkg/errors"
)

func TestVolumeRemoveErrors(t *testing.T) {
	testCases := []struct {
		args             []string
		volumeRemoveFunc func(volumeID string, force bool) error
		expectedError    string
	}{
		{
			expectedError: "requires at least 1 argument",
		},
		{
			args: []string{"nodeID"},
			volumeRemoveFunc: func(volumeID string, force bool) error {
				return errors.Errorf("error removing the volume")
			},
			expectedError: "error removing the volume",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newRemoveCommand(
			test.NewFakeCli(&fakeClient{
				volumeRemoveFunc: tc.volumeRemoveFunc,
			}, buf))
		cmd.SetArgs(tc.args)
		cmd.SetOutput(ioutil.Discard)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNodeRemoveMultiple(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newRemoveCommand(test.NewFakeCli(&fakeClient{}, buf))
	cmd.SetArgs([]string{"volume1", "volume2"})
	assert.NilError(t, cmd.Execute())
}
