package secret

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/pkg/errors"
)

func TestSecretRemoveErrors(t *testing.T) {
	testCases := []struct {
		args             []string
		secretRemoveFunc func(string) error
		expectedError    string
	}{
		{
			args:          []string{},
			expectedError: "requires at least 1 argument(s).",
		},
		{
			args: []string{"foo"},
			secretRemoveFunc: func(name string) error {
				return errors.Errorf("error removing secret")
			},
			expectedError: "error removing secret",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newSecretRemoveCommand(
			test.NewFakeCli(&fakeClient{
				secretRemoveFunc: tc.secretRemoveFunc,
			}, buf),
		)
		cmd.SetArgs(tc.args)
		cmd.SetOutput(ioutil.Discard)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestSecretRemoveWithName(t *testing.T) {
	names := []string{"foo", "bar"}
	buf := new(bytes.Buffer)
	var removedSecrets []string
	cli := test.NewFakeCli(&fakeClient{
		secretRemoveFunc: func(name string) error {
			removedSecrets = append(removedSecrets, name)
			return nil
		},
	}, buf)
	cmd := newSecretRemoveCommand(cli)
	cmd.SetArgs(names)
	assert.NilError(t, cmd.Execute())
	assert.EqualStringSlice(t, strings.Split(strings.TrimSpace(buf.String()), "\n"), names)
	assert.EqualStringSlice(t, removedSecrets, names)
}

func TestSecretRemoveContinueAfterError(t *testing.T) {
	names := []string{"foo", "bar"}
	buf := new(bytes.Buffer)
	var removedSecrets []string

	cli := test.NewFakeCli(&fakeClient{
		secretRemoveFunc: func(name string) error {
			removedSecrets = append(removedSecrets, name)
			if name == "foo" {
				return errors.Errorf("error removing secret: %s", name)
			}
			return nil
		},
	}, buf)

	cmd := newSecretRemoveCommand(cli)
	cmd.SetArgs(names)
	assert.Error(t, cmd.Execute(), "error removing secret: foo")
	assert.EqualStringSlice(t, removedSecrets, names)
}
