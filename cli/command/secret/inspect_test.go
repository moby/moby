package secret

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/internal/test"
	"github.com/pkg/errors"
	// Import builders to get the builder function as package function
	. "github.com/docker/docker/cli/internal/test/builders"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/docker/pkg/testutil/golden"
)

func TestSecretInspectErrors(t *testing.T) {
	testCases := []struct {
		args              []string
		flags             map[string]string
		secretInspectFunc func(secretID string) (swarm.Secret, []byte, error)
		expectedError     string
	}{
		{
			expectedError: "requires at least 1 argument",
		},
		{
			args: []string{"foo"},
			secretInspectFunc: func(secretID string) (swarm.Secret, []byte, error) {
				return swarm.Secret{}, nil, errors.Errorf("error while inspecting the secret")
			},
			expectedError: "error while inspecting the secret",
		},
		{
			args: []string{"foo"},
			flags: map[string]string{
				"format": "{{invalid format}}",
			},
			expectedError: "Template parsing error",
		},
		{
			args: []string{"foo", "bar"},
			secretInspectFunc: func(secretID string) (swarm.Secret, []byte, error) {
				if secretID == "foo" {
					return *Secret(SecretName("foo")), nil, nil
				}
				return swarm.Secret{}, nil, errors.Errorf("error while inspecting the secret")
			},
			expectedError: "error while inspecting the secret",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newSecretInspectCommand(
			test.NewFakeCli(&fakeClient{
				secretInspectFunc: tc.secretInspectFunc,
			}, buf),
		)
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		cmd.SetOutput(ioutil.Discard)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestSecretInspectWithoutFormat(t *testing.T) {
	testCases := []struct {
		name              string
		args              []string
		secretInspectFunc func(secretID string) (swarm.Secret, []byte, error)
	}{
		{
			name: "single-secret",
			args: []string{"foo"},
			secretInspectFunc: func(name string) (swarm.Secret, []byte, error) {
				if name != "foo" {
					return swarm.Secret{}, nil, errors.Errorf("Invalid name, expected %s, got %s", "foo", name)
				}
				return *Secret(SecretID("ID-foo"), SecretName("foo")), nil, nil
			},
		},
		{
			name: "multiple-secrets-with-labels",
			args: []string{"foo", "bar"},
			secretInspectFunc: func(name string) (swarm.Secret, []byte, error) {
				return *Secret(SecretID("ID-"+name), SecretName(name), SecretLabels(map[string]string{
					"label1": "label-foo",
				})), nil, nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newSecretInspectCommand(
			test.NewFakeCli(&fakeClient{
				secretInspectFunc: tc.secretInspectFunc,
			}, buf),
		)
		cmd.SetArgs(tc.args)
		assert.NilError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), fmt.Sprintf("secret-inspect-without-format.%s.golden", tc.name))
		assert.EqualNormalizedString(t, assert.RemoveSpace, actual, string(expected))
	}
}

func TestSecretInspectWithFormat(t *testing.T) {
	secretInspectFunc := func(name string) (swarm.Secret, []byte, error) {
		return *Secret(SecretName("foo"), SecretLabels(map[string]string{
			"label1": "label-foo",
		})), nil, nil
	}
	testCases := []struct {
		name              string
		format            string
		args              []string
		secretInspectFunc func(name string) (swarm.Secret, []byte, error)
	}{
		{
			name:              "simple-template",
			format:            "{{.Spec.Name}}",
			args:              []string{"foo"},
			secretInspectFunc: secretInspectFunc,
		},
		{
			name:              "json-template",
			format:            "{{json .Spec.Labels}}",
			args:              []string{"foo"},
			secretInspectFunc: secretInspectFunc,
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newSecretInspectCommand(
			test.NewFakeCli(&fakeClient{
				secretInspectFunc: tc.secretInspectFunc,
			}, buf),
		)
		cmd.SetArgs(tc.args)
		cmd.Flags().Set("format", tc.format)
		assert.NilError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), fmt.Sprintf("secret-inspect-with-format.%s.golden", tc.name))
		assert.EqualNormalizedString(t, assert.RemoveSpace, actual, string(expected))
	}
}
