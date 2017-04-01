package secret

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/config/configfile"
	"github.com/docker/docker/cli/internal/test"
	"github.com/pkg/errors"
	// Import builders to get the builder function as package function
	. "github.com/docker/docker/cli/internal/test/builders"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/docker/pkg/testutil/golden"
)

func TestSecretListErrors(t *testing.T) {
	testCases := []struct {
		args           []string
		secretListFunc func(types.SecretListOptions) ([]swarm.Secret, error)
		expectedError  string
	}{
		{
			args:          []string{"foo"},
			expectedError: "accepts no argument",
		},
		{
			secretListFunc: func(options types.SecretListOptions) ([]swarm.Secret, error) {
				return []swarm.Secret{}, errors.Errorf("error listing secrets")
			},
			expectedError: "error listing secrets",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newSecretListCommand(
			test.NewFakeCli(&fakeClient{
				secretListFunc: tc.secretListFunc,
			}, buf),
		)
		cmd.SetArgs(tc.args)
		cmd.SetOutput(ioutil.Discard)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestSecretList(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := test.NewFakeCli(&fakeClient{
		secretListFunc: func(options types.SecretListOptions) ([]swarm.Secret, error) {
			return []swarm.Secret{
				*Secret(SecretID("ID-foo"),
					SecretName("foo"),
					SecretVersion(swarm.Version{Index: 10}),
					SecretCreatedAt(time.Now().Add(-2*time.Hour)),
					SecretUpdatedAt(time.Now().Add(-1*time.Hour)),
				),
				*Secret(SecretID("ID-bar"),
					SecretName("bar"),
					SecretVersion(swarm.Version{Index: 11}),
					SecretCreatedAt(time.Now().Add(-2*time.Hour)),
					SecretUpdatedAt(time.Now().Add(-1*time.Hour)),
				),
			}, nil
		},
	}, buf)
	cli.SetConfigfile(&configfile.ConfigFile{})
	cmd := newSecretListCommand(cli)
	cmd.SetOutput(buf)
	assert.NilError(t, cmd.Execute())
	actual := buf.String()
	expected := golden.Get(t, []byte(actual), "secret-list.golden")
	assert.EqualNormalizedString(t, assert.RemoveSpace, actual, string(expected))
}

func TestSecretListWithQuietOption(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := test.NewFakeCli(&fakeClient{
		secretListFunc: func(options types.SecretListOptions) ([]swarm.Secret, error) {
			return []swarm.Secret{
				*Secret(SecretID("ID-foo"), SecretName("foo")),
				*Secret(SecretID("ID-bar"), SecretName("bar"), SecretLabels(map[string]string{
					"label": "label-bar",
				})),
			}, nil
		},
	}, buf)
	cli.SetConfigfile(&configfile.ConfigFile{})
	cmd := newSecretListCommand(cli)
	cmd.Flags().Set("quiet", "true")
	assert.NilError(t, cmd.Execute())
	actual := buf.String()
	expected := golden.Get(t, []byte(actual), "secret-list-with-quiet-option.golden")
	assert.EqualNormalizedString(t, assert.RemoveSpace, actual, string(expected))
}

func TestSecretListWithConfigFormat(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := test.NewFakeCli(&fakeClient{
		secretListFunc: func(options types.SecretListOptions) ([]swarm.Secret, error) {
			return []swarm.Secret{
				*Secret(SecretID("ID-foo"), SecretName("foo")),
				*Secret(SecretID("ID-bar"), SecretName("bar"), SecretLabels(map[string]string{
					"label": "label-bar",
				})),
			}, nil
		},
	}, buf)
	cli.SetConfigfile(&configfile.ConfigFile{
		SecretFormat: "{{ .Name }} {{ .Labels }}",
	})
	cmd := newSecretListCommand(cli)
	assert.NilError(t, cmd.Execute())
	actual := buf.String()
	expected := golden.Get(t, []byte(actual), "secret-list-with-config-format.golden")
	assert.EqualNormalizedString(t, assert.RemoveSpace, actual, string(expected))
}

func TestSecretListWithFormat(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := test.NewFakeCli(&fakeClient{
		secretListFunc: func(options types.SecretListOptions) ([]swarm.Secret, error) {
			return []swarm.Secret{
				*Secret(SecretID("ID-foo"), SecretName("foo")),
				*Secret(SecretID("ID-bar"), SecretName("bar"), SecretLabels(map[string]string{
					"label": "label-bar",
				})),
			}, nil
		},
	}, buf)
	cmd := newSecretListCommand(cli)
	cmd.Flags().Set("format", "{{ .Name }} {{ .Labels }}")
	assert.NilError(t, cmd.Execute())
	actual := buf.String()
	expected := golden.Get(t, []byte(actual), "secret-list-with-format.golden")
	assert.EqualNormalizedString(t, assert.RemoveSpace, actual, string(expected))
}

func TestSecretListWithFilter(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := test.NewFakeCli(&fakeClient{
		secretListFunc: func(options types.SecretListOptions) ([]swarm.Secret, error) {
			assert.Equal(t, options.Filters.Get("name")[0], "foo")
			assert.Equal(t, options.Filters.Get("label")[0], "lbl1=Label-bar")
			return []swarm.Secret{
				*Secret(SecretID("ID-foo"),
					SecretName("foo"),
					SecretVersion(swarm.Version{Index: 10}),
					SecretCreatedAt(time.Now().Add(-2*time.Hour)),
					SecretUpdatedAt(time.Now().Add(-1*time.Hour)),
				),
				*Secret(SecretID("ID-bar"),
					SecretName("bar"),
					SecretVersion(swarm.Version{Index: 11}),
					SecretCreatedAt(time.Now().Add(-2*time.Hour)),
					SecretUpdatedAt(time.Now().Add(-1*time.Hour)),
				),
			}, nil
		},
	}, buf)
	cli.SetConfigfile(&configfile.ConfigFile{})
	cmd := newSecretListCommand(cli)
	cmd.Flags().Set("filter", "name=foo")
	cmd.Flags().Set("filter", "label=lbl1=Label-bar")
	assert.NilError(t, cmd.Execute())
	actual := buf.String()
	expected := golden.Get(t, []byte(actual), "secret-list-with-filter.golden")
	assert.EqualNormalizedString(t, assert.RemoveSpace, actual, string(expected))
}
