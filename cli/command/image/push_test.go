package image

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil"
	"github.com/docker/docker/registry"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestNewPushCommandErrors(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		expectedError string
		imagePushFunc func(ref string, options types.ImagePushOptions) (io.ReadCloser, error)
	}{
		{
			name:          "wrong-args",
			args:          []string{},
			expectedError: "requires exactly 1 argument(s).",
		},
		{
			name:          "invalid-name",
			args:          []string{"UPPERCASE_REPO"},
			expectedError: "invalid reference format: repository name must be lowercase",
		},
		{
			name:          "push-failed",
			args:          []string{"image:repo"},
			expectedError: "Failed to push",
			imagePushFunc: func(ref string, options types.ImagePushOptions) (io.ReadCloser, error) {
				return ioutil.NopCloser(strings.NewReader("")), errors.Errorf("Failed to push")
			},
		},
		{
			name:          "trust-error",
			args:          []string{"--disable-content-trust=false", "image:repo"},
			expectedError: "you are not authorized to perform this operation: server returned 401.",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := NewPushCommand(test.NewFakeCli(&fakeClient{imagePushFunc: tc.imagePushFunc}, buf))
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNewPushCommandSuccess(t *testing.T) {
	testCases := []struct {
		name            string
		args            []string
		trustedPushFunc func(ctx context.Context, cli command.Cli, repoInfo *registry.RepositoryInfo,
			ref reference.Named, authConfig types.AuthConfig,
			requestPrivilege types.RequestPrivilegeFunc) error
	}{
		{
			name: "simple",
			args: []string{"image:tag"},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := NewPushCommand(test.NewFakeCli(&fakeClient{
			imagePushFunc: func(ref string, options types.ImagePushOptions) (io.ReadCloser, error) {
				return ioutil.NopCloser(strings.NewReader("")), nil
			},
		}, buf))
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		assert.NoError(t, cmd.Execute())
	}
}
