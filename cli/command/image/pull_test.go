package image

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil"
	"github.com/docker/docker/pkg/testutil/golden"
	"github.com/docker/docker/registry"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestNewPullCommandErrors(t *testing.T) {
	testCases := []struct {
		name            string
		args            []string
		expectedError   string
		trustedPullFunc func(ctx context.Context, cli command.Cli, repoInfo *registry.RepositoryInfo, ref reference.Named,
			authConfig types.AuthConfig, requestPrivilege types.RequestPrivilegeFunc) error
	}{
		{
			name:          "wrong-args",
			expectedError: "requires exactly 1 argument(s).",
			args:          []string{},
		},
		{
			name:          "invalid-name",
			expectedError: "invalid reference format: repository name must be lowercase",
			args:          []string{"UPPERCASE_REPO"},
		},
		{
			name:          "all-tags-with-tag",
			expectedError: "tag can't be used with --all-tags/-a",
			args:          []string{"--all-tags", "image:tag"},
		},
		{
			name:          "pull-error",
			args:          []string{"--disable-content-trust=false", "image:tag"},
			expectedError: "you are not authorized to perform this operation: server returned 401.",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := NewPullCommand(test.NewFakeCli(&fakeClient{}, buf))
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNewPullCommandSuccess(t *testing.T) {
	testCases := []struct {
		name            string
		args            []string
		trustedPullFunc func(ctx context.Context, cli command.Cli, repoInfo *registry.RepositoryInfo, ref reference.Named,
			authConfig types.AuthConfig, requestPrivilege types.RequestPrivilegeFunc) error
	}{
		{
			name: "simple",
			args: []string{"image:tag"},
		},
		{
			name: "simple-no-tag",
			args: []string{"image"},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := NewPullCommand(test.NewFakeCli(&fakeClient{}, buf))
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		err := cmd.Execute()
		assert.NoError(t, err)
		actual := buf.String()
		expected := string(golden.Get(t, []byte(actual), fmt.Sprintf("pull-command-success.%s.golden", tc.name))[:])
		testutil.EqualNormalizedString(t, testutil.RemoveSpace, actual, expected)
	}
}
