package service

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

// parseSecrets retrieves the secrets from the requested names and converts
// them to secret references to use with the spec
func parseSecrets(client client.APIClient, requestedSecrets []*SecretRequestSpec) ([]*swarmtypes.SecretReference, error) {
	secretRefs := make(map[string]*swarmtypes.SecretReference)
	ctx := context.Background()

	for _, secret := range requestedSecrets {
		secretRef := &swarmtypes.SecretReference{
			SecretName: secret.source,
			Target: swarmtypes.SecretReferenceFileTarget{
				Name: secret.target,
				UID:  secret.uid,
				GID:  secret.gid,
				Mode: secret.mode,
			},
		}

		if _, exists := secretRefs[secret.target]; exists {
			return nil, fmt.Errorf("duplicate secret target for %s not allowed", secret.source)
		}
		secretRefs[secret.target] = secretRef
	}

	args := filters.NewArgs()
	for _, s := range secretRefs {
		args.Add("names", s.SecretName)
	}

	secrets, err := client.SecretList(ctx, types.SecretListOptions{
		Filter: args,
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
			return nil, fmt.Errorf("secret not found: %s", ref.SecretName)
		}

		// set the id for the ref to properly assign in swarm
		// since swarm needs the ID instead of the name
		ref.SecretID = id
		addedSecrets = append(addedSecrets, ref)
	}

	return addedSecrets, nil
}
