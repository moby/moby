package image

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil"
	"github.com/docker/docker/pkg/testutil/golden"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewLoadCommandErrors(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		isTerminalIn  bool
		expectedError string
		imageLoadFunc func(input io.Reader, quiet bool) (types.ImageLoadResponse, error)
	}{
		{
			name:          "wrong-args",
			args:          []string{"arg"},
			expectedError: "accepts no argument(s).",
		},
		{
			name:          "input-to-terminal",
			isTerminalIn:  true,
			expectedError: "requested load from stdin, but stdin is empty",
		},
		{
			name:          "pull-error",
			expectedError: "something went wrong",
			imageLoadFunc: func(input io.Reader, quiet bool) (types.ImageLoadResponse, error) {
				return types.ImageLoadResponse{}, errors.Errorf("something went wrong")
			},
		},
	}
	for _, tc := range testCases {
		cli := test.NewFakeCli(&fakeClient{imageLoadFunc: tc.imageLoadFunc}, new(bytes.Buffer))
		cli.In().SetIsTerminal(tc.isTerminalIn)
		cmd := NewLoadCommand(cli)
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNewLoadCommandInvalidInput(t *testing.T) {
	expectedError := "open *"
	cmd := NewLoadCommand(test.NewFakeCli(&fakeClient{}, new(bytes.Buffer)))
	cmd.SetOutput(ioutil.Discard)
	cmd.SetArgs([]string{"--input", "*"})
	err := cmd.Execute()
	testutil.ErrorContains(t, err, expectedError)
}

func TestNewLoadCommandSuccess(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		imageLoadFunc func(input io.Reader, quiet bool) (types.ImageLoadResponse, error)
	}{
		{
			name: "simple",
			imageLoadFunc: func(input io.Reader, quiet bool) (types.ImageLoadResponse, error) {
				return types.ImageLoadResponse{Body: ioutil.NopCloser(strings.NewReader("Success"))}, nil
			},
		},
		{
			name: "json",
			imageLoadFunc: func(input io.Reader, quiet bool) (types.ImageLoadResponse, error) {
				json := "{\"ID\": \"1\"}"
				return types.ImageLoadResponse{
					Body: ioutil.NopCloser(strings.NewReader(json)),
					JSON: true,
				}, nil
			},
		},
		{
			name: "input-file",
			args: []string{"--input", "testdata/load-command-success.input.txt"},
			imageLoadFunc: func(input io.Reader, quiet bool) (types.ImageLoadResponse, error) {
				return types.ImageLoadResponse{Body: ioutil.NopCloser(strings.NewReader("Success"))}, nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := NewLoadCommand(test.NewFakeCli(&fakeClient{imageLoadFunc: tc.imageLoadFunc}, buf))
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		err := cmd.Execute()
		assert.NoError(t, err)
		actual := buf.String()
		expected := string(golden.Get(t, []byte(actual), fmt.Sprintf("load-command-success.%s.golden", tc.name))[:])
		testutil.EqualNormalizedString(t, testutil.RemoveSpace, actual, expected)
	}
}
