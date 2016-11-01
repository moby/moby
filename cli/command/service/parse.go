package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

// parseSecretString parses the requested secret and returns the secret name
// and target.  Expects format SECRET_NAME:TARGET
func parseSecretString(secretString string) (string, string, error) {
	tokens := strings.Split(secretString, ":")

	secretName := strings.TrimSpace(tokens[0])
	targetName := secretName

	if secretName == "" {
		return "", "", fmt.Errorf("invalid secret name provided")
	}

	if len(tokens) > 1 {
		targetName = strings.TrimSpace(tokens[1])
		if targetName == "" {
			return "", "", fmt.Errorf("invalid presentation name provided")
		}
	}

	// ensure target is a filename only; no paths allowed
	tDir, _ := filepath.Split(targetName)
	if tDir != "" {
		return "", "", fmt.Errorf("target must not have a path")
	}

	return secretName, targetName, nil
}

// parseSecrets retrieves the secrets from the requested names and converts
// them to secret references to use with the spec
func parseSecrets(client client.APIClient, requestedSecrets []string) ([]*swarmtypes.SecretReference, error) {
	secretRefs := make(map[string]*swarmtypes.SecretReference)
	ctx := context.Background()

	for _, secret := range requestedSecrets {
		n, t, err := parseSecretString(secret)
		if err != nil {
			return nil, err
		}

		secretRef := &swarmtypes.SecretReference{
			SecretName: n,
			// TODO (ehazlett): parse these from cli request
			Target: swarmtypes.SecretReferenceFileTarget{
				Name: t,
				UID:  "0",
				GID:  "0",
				Mode: 0444,
			},
		}

		if _, exists := secretRefs[t]; exists {
			return nil, fmt.Errorf("duplicate secret target for %s not allowed", n)
		}
		secretRefs[t] = secretRef
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
