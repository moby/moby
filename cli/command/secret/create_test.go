package secret

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil"
	"github.com/docker/docker/pkg/testutil/golden"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

const secretDataFile = "secret-create-with-name.golden"

func TestSecretCreateErrors(t *testing.T) {
	testCases := []struct {
		args             []string
		secretCreateFunc func(swarm.SecretSpec) (types.SecretCreateResponse, error)
		expectedError    string
	}{
		{
			args:          []string{"too_few"},
			expectedError: "requires exactly 2 argument(s)",
		},
		{args: []string{"too", "many", "arguments"},
			expectedError: "requires exactly 2 argument(s)",
		},
		{
			args: []string{"name", filepath.Join("testdata", secretDataFile)},
			secretCreateFunc: func(secretSpec swarm.SecretSpec) (types.SecretCreateResponse, error) {
				return types.SecretCreateResponse{}, errors.Errorf("error creating secret")
			},
			expectedError: "error creating secret",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newSecretCreateCommand(
			test.NewFakeCli(&fakeClient{
				secretCreateFunc: tc.secretCreateFunc,
			}, buf),
		)
		cmd.SetArgs(tc.args)
		cmd.SetOutput(ioutil.Discard)
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestSecretCreateWithName(t *testing.T) {
	name := "foo"
	buf := new(bytes.Buffer)
	var actual []byte
	cli := test.NewFakeCli(&fakeClient{
		secretCreateFunc: func(spec swarm.SecretSpec) (types.SecretCreateResponse, error) {
			if spec.Name != name {
				return types.SecretCreateResponse{}, errors.Errorf("expected name %q, got %q", name, spec.Name)
			}

			actual = spec.Data

			return types.SecretCreateResponse{
				ID: "ID-" + spec.Name,
			}, nil
		},
	}, buf)

	cmd := newSecretCreateCommand(cli)
	cmd.SetArgs([]string{name, filepath.Join("testdata", secretDataFile)})
	assert.NoError(t, cmd.Execute())
	expected := golden.Get(t, actual, secretDataFile)
	assert.Equal(t, expected, actual)
	assert.Equal(t, "ID-"+name, strings.TrimSpace(buf.String()))
}

func TestSecretCreateWithLabels(t *testing.T) {
	expectedLabels := map[string]string{
		"lbl1": "Label-foo",
		"lbl2": "Label-bar",
	}
	name := "foo"

	buf := new(bytes.Buffer)
	cli := test.NewFakeCli(&fakeClient{
		secretCreateFunc: func(spec swarm.SecretSpec) (types.SecretCreateResponse, error) {
			if spec.Name != name {
				return types.SecretCreateResponse{}, errors.Errorf("expected name %q, got %q", name, spec.Name)
			}

			if !compareMap(spec.Labels, expectedLabels) {
				return types.SecretCreateResponse{}, errors.Errorf("expected labels %v, got %v", expectedLabels, spec.Labels)
			}

			return types.SecretCreateResponse{
				ID: "ID-" + spec.Name,
			}, nil
		},
	}, buf)

	cmd := newSecretCreateCommand(cli)
	cmd.SetArgs([]string{name, filepath.Join("testdata", secretDataFile)})
	cmd.Flags().Set("label", "lbl1=Label-foo")
	cmd.Flags().Set("label", "lbl2=Label-bar")
	assert.NoError(t, cmd.Execute())
	assert.Equal(t, "ID-"+name, strings.TrimSpace(buf.String()))
}

func compareMap(actual map[string]string, expected map[string]string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for key, value := range actual {
		if expectedValue, ok := expected[key]; ok {
			if expectedValue != value {
				return false
			}
		} else {
			return false
		}
	}
	return true
}
