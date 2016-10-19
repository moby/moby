package service

import (
	"context"
	"fmt"
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
	targetName := ""

	if secretName == "" {
		return "", "", fmt.Errorf("invalid secret name provided")
	}

	if len(tokens) > 1 {
		targetName = strings.TrimSpace(tokens[1])
		if targetName == "" {
			return "", "", fmt.Errorf("invalid presentation name provided")
		}
	} else {
		targetName = secretName
	}
	return secretName, targetName, nil
}

// parseSecrets retrieves the secrets from the requested names and converts
// them to secret references to use with the spec
func parseSecrets(client client.APIClient, requestedSecrets []string) ([]*swarmtypes.SecretReference, error) {
	lookupSecretNames := []string{}
	needSecrets := make(map[string]*swarmtypes.SecretReference)
	ctx := context.Background()

	for _, secret := range requestedSecrets {
		n, t, err := parseSecretString(secret)
		if err != nil {
			return nil, err
		}

		secretRef := &swarmtypes.SecretReference{
			SecretName: n,
			Mode:       swarmtypes.SecretReferenceFile,
			Target:     t,
		}

		lookupSecretNames = append(lookupSecretNames, n)
		needSecrets[n] = secretRef
	}

	args := filters.NewArgs()
	for _, s := range lookupSecretNames {
		args.Add("names", s)
	}

	secrets, err := client.SecretList(ctx, types.SecretListOptions{
		Filter: args,
	})
	if err != nil {
		return nil, err
	}

	foundSecrets := make(map[string]*swarmtypes.Secret)
	for _, secret := range secrets {
		foundSecrets[secret.Spec.Annotations.Name] = &secret
	}

	addedSecrets := []*swarmtypes.SecretReference{}

	for secretName, secretRef := range needSecrets {
		s, ok := foundSecrets[secretName]
		if !ok {
			return nil, fmt.Errorf("secret not found: %s", secretName)
		}

		// set the id for the ref to properly assign in swarm
		// since swarm needs the ID instead of the name
		secretRef.SecretID = s.ID
		addedSecrets = append(addedSecrets, secretRef)
	}

	return addedSecrets, nil
}
