package stack

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/cli/internal/test"
	"github.com/stretchr/testify/assert"
)

func TestRemoveStack(t *testing.T) {
	allServices := []string{
		objectName("foo", "service1"),
		objectName("foo", "service2"),
		objectName("bar", "service1"),
		objectName("bar", "service2"),
	}
	allServiceIDs := buildObjectIDs(allServices)

	allNetworks := []string{
		objectName("foo", "network1"),
		objectName("bar", "network1"),
	}
	allNetworkIDs := buildObjectIDs(allNetworks)

	allSecrets := []string{
		objectName("foo", "secret1"),
		objectName("foo", "secret2"),
		objectName("bar", "secret1"),
	}
	allSecretIDs := buildObjectIDs(allSecrets)

	cli := &fakeClient{
		services: allServices,
		networks: allNetworks,
		secrets:  allSecrets,
	}
	cmd := newRemoveCommand(test.NewFakeCli(cli, &bytes.Buffer{}))
	cmd.SetArgs([]string{"foo", "bar"})

	assert.NoError(t, cmd.Execute())
	assert.Equal(t, allServiceIDs, cli.removedServices)
	assert.Equal(t, allNetworkIDs, cli.removedNetworks)
	assert.Equal(t, allSecretIDs, cli.removedSecrets)
}

func TestSkipEmptyStack(t *testing.T) {
	buf := new(bytes.Buffer)
	allServices := []string{objectName("bar", "service1"), objectName("bar", "service2")}
	allServiceIDs := buildObjectIDs(allServices)

	allNetworks := []string{objectName("bar", "network1")}
	allNetworkIDs := buildObjectIDs(allNetworks)

	allSecrets := []string{objectName("bar", "secret1")}
	allSecretIDs := buildObjectIDs(allSecrets)

	cli := &fakeClient{
		services: allServices,
		networks: allNetworks,
		secrets:  allSecrets,
	}
	cmd := newRemoveCommand(test.NewFakeCli(cli, buf))
	cmd.SetArgs([]string{"foo", "bar"})

	assert.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Nothing found in stack: foo")
	assert.Equal(t, allServiceIDs, cli.removedServices)
	assert.Equal(t, allNetworkIDs, cli.removedNetworks)
	assert.Equal(t, allSecretIDs, cli.removedSecrets)
}

func TestContinueAfterError(t *testing.T) {
	allServices := []string{objectName("foo", "service1"), objectName("bar", "service1")}
	allServiceIDs := buildObjectIDs(allServices)

	allNetworks := []string{objectName("foo", "network1"), objectName("bar", "network1")}
	allNetworkIDs := buildObjectIDs(allNetworks)

	allSecrets := []string{objectName("foo", "secret1"), objectName("bar", "secret1")}
	allSecretIDs := buildObjectIDs(allSecrets)

	removedServices := []string{}
	cli := &fakeClient{
		services: allServices,
		networks: allNetworks,
		secrets:  allSecrets,

		serviceRemoveFunc: func(serviceID string) error {
			removedServices = append(removedServices, serviceID)

			if strings.Contains(serviceID, "foo") {
				return errors.New("")
			}
			return nil
		},
	}
	cmd := newRemoveCommand(test.NewFakeCli(cli, &bytes.Buffer{}))
	cmd.SetArgs([]string{"foo", "bar"})

	assert.EqualError(t, cmd.Execute(), "Failed to remove some resources from stack: foo")
	assert.Equal(t, allServiceIDs, removedServices)
	assert.Equal(t, allNetworkIDs, cli.removedNetworks)
	assert.Equal(t, allSecretIDs, cli.removedSecrets)
}
