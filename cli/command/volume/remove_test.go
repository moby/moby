package volume

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
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
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNodeRemoveMultiple(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newRemoveCommand(test.NewFakeCli(&fakeClient{}, buf))
	cmd.SetArgs([]string{"volume1", "volume2"})
	assert.NoError(t, cmd.Execute())
}
