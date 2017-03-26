package stack

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestRemoveStack(t *testing.T) {
	allServices := []string{
		objectName("foo", "service1"),
		objectName("foo", "service2"),
		objectName("bar", "service1"),
		objectName("bar", "service2"),
	}
	allServicesIDs := buildObjectIDs(allServices)

	allNetworks := []string{
		objectName("foo", "network1"),
		objectName("bar", "network1"),
	}
	allNetworksIDs := buildObjectIDs(allNetworks)

	allSecrets := []string{
		objectName("foo", "secret1"),
		objectName("foo", "secret2"),
		objectName("bar", "secret1"),
	}
	allSecretsIDs := buildObjectIDs(allSecrets)

	cli := &fakeClient{
		services: allServices,
		networks: allNetworks,
		secrets:  allSecrets,
	}
	cmd := newRemoveCommand(test.NewFakeCli(cli, &bytes.Buffer{}))
	cmd.SetArgs([]string{"foo", "bar"})

	assert.NilError(t, cmd.Execute())
	assert.DeepEqual(t, cli.removedServices, allServicesIDs)
	assert.DeepEqual(t, cli.removedNetworks, allNetworksIDs)
	assert.DeepEqual(t, cli.removedSecrets, allSecretsIDs)
}

func TestSkipEmptyStack(t *testing.T) {
	buf := new(bytes.Buffer)
	allServices := []string{objectName("bar", "service1"), objectName("bar", "service2")}
	allServicesIDs := buildObjectIDs(allServices)

	allNetworks := []string{objectName("bar", "network1")}
	allNetworksIDs := buildObjectIDs(allNetworks)

	allSecrets := []string{objectName("bar", "secret1")}
	allSecretsIDs := buildObjectIDs(allSecrets)

	cli := &fakeClient{
		services: allServices,
		networks: allNetworks,
		secrets:  allSecrets,
	}
	cmd := newRemoveCommand(test.NewFakeCli(cli, buf))
	cmd.SetArgs([]string{"foo", "bar"})

	assert.NilError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Nothing found in stack: foo")
	assert.DeepEqual(t, cli.removedServices, allServicesIDs)
	assert.DeepEqual(t, cli.removedNetworks, allNetworksIDs)
	assert.DeepEqual(t, cli.removedSecrets, allSecretsIDs)
}

func TestContinueAfterError(t *testing.T) {
	allServices := []string{objectName("foo", "service1"), objectName("bar", "service1")}
	allServicesIDs := buildObjectIDs(allServices)

	allNetworks := []string{objectName("foo", "network1"), objectName("bar", "network1")}
	allNetworksIDs := buildObjectIDs(allNetworks)

	allSecrets := []string{objectName("foo", "secret1"), objectName("bar", "secret1")}
	allSecretsIDs := buildObjectIDs(allSecrets)

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

	assert.Error(t, cmd.Execute(), "Failed to remove some resources from stack: foo")
	assert.DeepEqual(t, removedServices, allServicesIDs)
	assert.DeepEqual(t, cli.removedNetworks, allNetworksIDs)
	assert.DeepEqual(t, cli.removedSecrets, allSecretsIDs)
}
