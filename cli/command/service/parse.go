package service

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// ParseSecrets retrieves the secrets with the requested names and fills
// secret IDs into the secret references.
func ParseSecrets(client client.SecretAPIClient, requestedSecrets []*swarmtypes.SecretReference) ([]*swarmtypes.SecretReference, error) {
	secretRefs := make(map[string]*swarmtypes.SecretReference)
	ctx := context.Background()

	for _, secret := range requestedSecrets {
		if _, exists := secretRefs[secret.File.Name]; exists {
			return nil, errors.Errorf("duplicate secret target for %s not allowed", secret.SecretName)
		}
		secretRef := new(swarmtypes.SecretReference)
		*secretRef = *secret
		secretRefs[secret.File.Name] = secretRef
	}

	args := filters.NewArgs()
	for _, s := range secretRefs {
		args.Add("name", s.SecretName)
	}

	secrets, err := client.SecretList(ctx, types.SecretListOptions{
		Filters: args,
	})
	if err != nil {
		return nil, err
	}

	foundSecrets := make(map[string]string)
	for _, secret := range secrets {
		foundSecrets[secret.Spec.Annotations.Name] = secret.ID
	}

	addedSecrets := []*swarmtypes.SecretReference{}

	for _, ref := range secretRefs {
		id, ok := foundSecrets[ref.SecretName]
		if !ok {
			return nil, errors.Errorf("secret not found: %s", ref.SecretName)
		}

		// set the id for the ref to properly assign in swarm
		// since swarm needs the ID instead of the name
		ref.SecretID = id
		addedSecrets = append(addedSecrets, ref)
	}

	return addedSecrets, nil
}
