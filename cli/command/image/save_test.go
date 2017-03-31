package image

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/cli/internal/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewSaveCommandErrors(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		isTerminal    bool
		expectedError string
		imageSaveFunc func(images []string) (io.ReadCloser, error)
	}{
		{
			name:          "wrong args",
			args:          []string{},
			expectedError: "requires at least 1 argument(s).",
		},
		{
			name:          "output to terminal",
			args:          []string{"output", "file", "arg1"},
			isTerminal:    true,
			expectedError: "Cowardly refusing to save to a terminal. Use the -o flag or redirect.",
		},
		{
			name:          "ImageSave fail",
			args:          []string{"arg1"},
			isTerminal:    false,
			expectedError: "error saving image",
			imageSaveFunc: func(images []string) (io.ReadCloser, error) {
				return ioutil.NopCloser(strings.NewReader("")), errors.Errorf("error saving image")
			},
		},
	}
	for _, tc := range testCases {
		cli := test.NewFakeCli(&fakeClient{imageSaveFunc: tc.imageSaveFunc}, new(bytes.Buffer))
		cli.Out().SetIsTerminal(tc.isTerminal)
		cmd := NewSaveCommand(cli)
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNewSaveCommandSuccess(t *testing.T) {
	testCases := []struct {
		args          []string
		isTerminal    bool
		imageSaveFunc func(images []string) (io.ReadCloser, error)
		deferredFunc  func()
	}{
		{
			args:       []string{"-o", "save_tmp_file", "arg1"},
			isTerminal: true,
			imageSaveFunc: func(images []string) (io.ReadCloser, error) {
				assert.Equal(t, len(images), 1)
				assert.Equal(t, images[0], "arg1")
				return ioutil.NopCloser(strings.NewReader("")), nil
			},
			deferredFunc: func() {
				os.Remove("save_tmp_file")
			},
		},
		{
			args:       []string{"arg1", "arg2"},
			isTerminal: false,
			imageSaveFunc: func(images []string) (io.ReadCloser, error) {
				assert.Equal(t, len(images), 2)
				assert.Equal(t, images[0], "arg1")
				assert.Equal(t, images[1], "arg2")
				return ioutil.NopCloser(strings.NewReader("")), nil
			},
		},
	}
	for _, tc := range testCases {
		cmd := NewSaveCommand(test.NewFakeCli(&fakeClient{
			imageSaveFunc: func(images []string) (io.ReadCloser, error) {
				return ioutil.NopCloser(strings.NewReader("")), nil
			},
		}, new(bytes.Buffer)))
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		assert.NoError(t, cmd.Execute())
		if tc.deferredFunc != nil {
			tc.deferredFunc()
		}
	}
}
